/*
Copyright 2015 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package gc

import (
	"context"
	goerrors "errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/shirou/gopsutil/disk"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
)

var (
	// ErrImageNotFound -
	ErrImageNotFound = goerrors.New("image not found")
)

// FsStats -
type FsStats struct {
	CapacityBytes  uint64 `json:"capacityBytes,omitempty"`
	AvailableBytes uint64 `json:"availableBytes,omitempty"`
}

// GetFsStats -
func GetFsStats(path string) (*disk.UsageStat, error) {
	return disk.Usage(path)
}

// ImageGCManager is an interface for managing lifecycle of all images.
// Implementation is thread-safe.
type ImageGCManager interface {
	// Start async garbage collection of images.
	Start()

	SetServiceImages(seviceImages []string)
}

// ImageGCPolicy is a policy for garbage collecting images. Policy defines an allowed band in
// which garbage collection will be run.
type ImageGCPolicy struct {
	// Any usage above this threshold will always trigger garbage collection.
	// This is the highest usage we will allow.
	HighThresholdPercent int

	// Any usage below this threshold will never trigger garbage collection.
	// This is the lowest threshold we will try to garbage collect to.
	LowThresholdPercent int

	// Minimum age at which an image can be garbage collected.
	MinAge time.Duration

	// ImageGCPeriod is the period for performing image garbage collection.
	ImageGCPeriod time.Duration
}

type realImageGCManager struct {
	dockerClient *client.Client

	// Records of images and their use.
	imageRecords     map[string]*imageRecord
	imageRecordsLock sync.Mutex

	// The image garbage collection policy in use.
	policy ImageGCPolicy

	// Track initialization
	initialized bool

	// sandbox image exempted from GC
	sandboxImage  string
	serviceImages []string
}

// Information about the images we track.
type imageRecord struct {
	// Time when this image was first detected.
	firstDetected time.Time

	// Time when we last saw this image being used.
	lastUsed time.Time

	// Size of the image in bytes.
	size int64
}

// NewImageGCManager instantiates a new ImageGCManager object.
func NewImageGCManager(dockerClient *client.Client, policy ImageGCPolicy, sandboxImage string) (ImageGCManager, error) {
	// Validate policy.
	if policy.HighThresholdPercent < 0 || policy.HighThresholdPercent > 100 {
		return nil, fmt.Errorf("invalid HighThresholdPercent %d, must be in range [0-100]", policy.HighThresholdPercent)
	}
	if policy.LowThresholdPercent < 0 || policy.LowThresholdPercent > 100 {
		return nil, fmt.Errorf("invalid LowThresholdPercent %d, must be in range [0-100]", policy.LowThresholdPercent)
	}
	if policy.LowThresholdPercent > policy.HighThresholdPercent {
		return nil, fmt.Errorf("LowThresholdPercent %d can not be higher than HighThresholdPercent %d", policy.LowThresholdPercent, policy.HighThresholdPercent)
	}
	im := &realImageGCManager{
		dockerClient: dockerClient,
		policy:       policy,
		imageRecords: make(map[string]*imageRecord),
		initialized:  false,
		sandboxImage: sandboxImage,
	}

	return im, nil
}

func (im *realImageGCManager) Start() {
	logrus.Infof("start image gc manager; image gc period: %f", im.policy.ImageGCPeriod.Seconds())
	go wait.Until(func() {
		// Initial detection make detected time "unknown" in the past.
		var ts time.Time
		if im.initialized {
			ts = time.Now()
		}
		_, err := im.detectImages(ts)
		if err != nil {
			logrus.Warningf("[imageGCManager] Failed to monitor images: %v", err)
		} else {
			im.initialized = true
		}
	}, im.policy.ImageGCPeriod, wait.NeverStop)

	prevImageGCFailed := false
	go wait.Until(func() {
		if err := im.GarbageCollect(); err != nil {
			if prevImageGCFailed {
				logrus.Errorf("Image garbage collection failed multiple times in a row: %v", err)
			} else {
				logrus.Errorf("Image garbage collection failed once. Stats initialization may not have completed yet: %v", err)
			}
			prevImageGCFailed = true
		} else {
			logrus.Debug("Image garbage collection succeeded")
		}
	}, im.policy.ImageGCPeriod, wait.NeverStop)
}

func (im *realImageGCManager) SetServiceImages(serviceImages []string) {
	logrus.Infof("set service images: %s", strings.Join(serviceImages, ","))
	im.serviceImages = serviceImages
}

func (im *realImageGCManager) detectImages(detectTime time.Time) (sets.String, error) {
	imagesInUse := sets.NewString()

	// copy service images
	serviceImages := make([]string, len(im.serviceImages))
	copy(serviceImages, im.serviceImages)

	// Always consider the container runtime pod sandbox image in use
	serviceImages = append(serviceImages, im.sandboxImage)
	for _, image := range serviceImages {
		imageRef, err := im.getImageRef(image)
		if err == nil && imageRef != "" {
			imagesInUse.Insert(imageRef)
		}
	}

	images, err := im.listImages()
	if err != nil {
		return imagesInUse, err
	}

	// Add new images and record those being used.
	now := time.Now()
	currentImages := sets.NewString()
	im.imageRecordsLock.Lock()
	defer im.imageRecordsLock.Unlock()
	for _, image := range images {
		logrus.Debugf("Adding image ID %s to currentImages", image.ID)
		currentImages.Insert(image.ID)

		// New image, set it as detected now.
		if _, ok := im.imageRecords[image.ID]; !ok {
			logrus.Debugf("Image ID %s is new", image.ID)
			im.imageRecords[image.ID] = &imageRecord{
				firstDetected: detectTime,
			}
		}

		// Set last used time to now if the image is being used.
		if isImageUsed(image.ID, imagesInUse) {
			logrus.Debugf("Setting Image ID %s lastUsed to %v", image.ID, now)
			im.imageRecords[image.ID].lastUsed = now
		}

		logrus.Debugf("Image ID %s has size %d", image.ID, image.Size)
		im.imageRecords[image.ID].size = image.Size
	}

	// Remove old images from our records.
	for image := range im.imageRecords {
		if !currentImages.Has(image) {
			logrus.Debugf("Image ID %s is no longer present; removing from imageRecords", image)
			delete(im.imageRecords, image)
		}
	}

	return imagesInUse, nil
}

func (im *realImageGCManager) getImageRef(imageID string) (string, error) {
	ctx, cancel := getContextWithTimeout(3 * time.Second)
	defer cancel()

	inspect, _, err := im.dockerClient.ImageInspectWithRaw(ctx, imageID)
	if err != nil {
		if strings.Contains(err.Error(), "No such image") {
			return "", ErrImageNotFound
		}
		return "", err
	}

	return inspect.ID, nil
}

func (im *realImageGCManager) listImages() ([]types.ImageSummary, error) {
	ctx, cancel := getContextWithTimeout(3 * time.Second)
	defer cancel()

	return im.dockerClient.ImageList(ctx, types.ImageListOptions{})
}

func (im *realImageGCManager) removeImage(imageID string) error {
	ctx, cancel := getContextWithTimeout(3 * time.Second)
	defer cancel()

	opts := types.ImageRemoveOptions{
		Force: true,
	}
	items, err := im.dockerClient.ImageRemove(ctx, imageID, opts)
	if err != nil {
		return err
	}

	for _, item := range items {
		if item.Deleted != "" {
			logrus.Debugf("image deleted: %s", item.Deleted)
		}
		if item.Untagged != "" {
			logrus.Debugf("image untagged: %s", item.Untagged)
		}
	}

	return nil
}

func (im *realImageGCManager) dockerRootDir() (string, error) {
	ctx, cancel := getContextWithTimeout(3 * time.Second)
	defer cancel()

	dockerInfo, err := im.dockerClient.Info(ctx)
	if err != nil {
		return "", fmt.Errorf("docker info: %v", err)
	}

	return dockerInfo.DockerRootDir, nil
}

// getContextWithTimeout returns a context with timeout.
func getContextWithTimeout(timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), timeout)
}

func (im *realImageGCManager) GarbageCollect() error {
	dockerRootDir, err := im.dockerRootDir()
	if err != nil {
		logrus.Errorf("failed to get docker root dir: %v; use '/var/lib/docker'", err)
		dockerRootDir = "/var/lib/docker"
	}

	logrus.Infof("docker root dir: %s", dockerRootDir)
	fsStats, err := GetFsStats(dockerRootDir)
	if err != nil {
		return err
	}

	available := fsStats.Free
	capacity := fsStats.Total
	if available > capacity {
		logrus.Warningf("available %d is larger than capacity %d", available, capacity)
		available = capacity
	}

	// Check valid capacity.
	if capacity == 0 {
		err := goerrors.New("invalid capacity 0 on image filesystem")
		return err
	}

	// If over the max threshold, free enough to place us at the lower threshold.
	usagePercent := fsStats.UsedPercent
	logrus.Infof("[imageGCManager]: available disk: %d bytes; capacity of disk: %d bytes; disk usage on image filesystem: %0.f%%; high threshold (%d%%).", available, capacity, usagePercent, im.policy.HighThresholdPercent)
	if usagePercent >= float64(im.policy.HighThresholdPercent) {
		amountToFree := int64(capacity)*int64(100-im.policy.LowThresholdPercent)/100 - int64(available)
		logrus.Infof("[imageGCManager]: Disk usage on image filesystem is at %0.f%% which is over the high threshold (%d%%). Trying to free %d bytes down to the low threshold (%d%%).", usagePercent, im.policy.HighThresholdPercent, amountToFree, im.policy.LowThresholdPercent)
		freed, err := im.freeSpace(amountToFree, time.Now())
		if err != nil {
			return err
		}

		if freed < amountToFree {
			logrus.Debugf("failed to garbage collect required amount of images. Wanted to free %d bytes, but freed %d bytes", amountToFree, freed)
			return fmt.Errorf("failed to garbage collect required amount of images. Wanted to free %d bytes, but freed %d bytes", amountToFree, freed)
		}
	}

	return nil
}

// Tries to free bytesToFree worth of images on the disk.
//
// Returns the number of bytes free and an error if any occurred. The number of
// bytes freed is always returned.
// Note that error may be nil and the number of bytes free may be less
// than bytesToFree.
func (im *realImageGCManager) freeSpace(bytesToFree int64, freeTime time.Time) (int64, error) {
	imagesInUse, err := im.detectImages(freeTime)
	if err != nil {
		return 0, err
	}

	im.imageRecordsLock.Lock()
	defer im.imageRecordsLock.Unlock()

	// Get all images in eviction order.
	images := make([]evictionInfo, 0, len(im.imageRecords))
	for image, record := range im.imageRecords {
		if isImageUsed(image, imagesInUse) {
			logrus.Debugf("Image ID %s is being used", image)
			continue
		}
		images = append(images, evictionInfo{
			id:          image,
			imageRecord: *record,
		})
	}
	sort.Sort(byLastUsedAndDetected(images))

	// Delete unused images until we've freed up enough space.
	var deletionErrors []error
	spaceFreed := int64(0)
	for _, image := range images {
		logrus.Debugf("Evaluating image ID %s for possible garbage collection", image.id)
		// Images that are currently in used were given a newer lastUsed.
		if image.lastUsed.Equal(freeTime) || image.lastUsed.After(freeTime) {
			logrus.Debugf("Image ID %s has lastUsed=%v which is >= freeTime=%v, not eligible for garbage collection", image.id, image.lastUsed, freeTime)
			continue
		}

		// Avoid garbage collect the image if the image is not old enough.
		// In such a case, the image may have just been pulled down, and will be used by a container right away.

		if freeTime.Sub(image.firstDetected) < im.policy.MinAge {
			logrus.Debugf("Image ID %s has age %v which is less than the policy's minAge of %v, not eligible for garbage collection", image.id, freeTime.Sub(image.firstDetected), im.policy.MinAge)
			continue
		}

		// Remove image. Continue despite errors.
		logrus.Debugf("[imageGCManager]: Removing image %q to free %d bytes", image.id, image.size)
		err := im.removeImage(image.id)
		if err != nil {
			continue
		}
		delete(im.imageRecords, image.id)
		spaceFreed += image.size

		if spaceFreed >= bytesToFree {
			logrus.Debugf("spaceFreed(%f) is greater than bytesToFree(%f), stop free space")
			break
		}
	}

	if len(deletionErrors) > 0 {
		return spaceFreed, fmt.Errorf("wanted to free %d bytes, but freed %d bytes space with errors in image deletion: %v", bytesToFree, spaceFreed, errors.NewAggregate(deletionErrors))
	}
	return spaceFreed, nil
}

type evictionInfo struct {
	id string
	imageRecord
}

type byLastUsedAndDetected []evictionInfo

func (ev byLastUsedAndDetected) Len() int      { return len(ev) }
func (ev byLastUsedAndDetected) Swap(i, j int) { ev[i], ev[j] = ev[j], ev[i] }
func (ev byLastUsedAndDetected) Less(i, j int) bool {
	// Sort by last used, break ties by detected.
	if ev[i].lastUsed.Equal(ev[j].lastUsed) {
		return ev[i].firstDetected.Before(ev[j].firstDetected)
	}
	return ev[i].lastUsed.Before(ev[j].lastUsed)
}

func isImageUsed(imageID string, imagesInUse sets.String) bool {
	// Check the image ID.
	if _, ok := imagesInUse[imageID]; ok {
		return true
	}
	return false
}
