package sources

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/containerd/containerd"
	ctrcontent "github.com/containerd/containerd/cmd/ctr/commands/content"
	"github.com/containerd/containerd/content"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/images/archive"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/pkg/progress"
	"github.com/containerd/containerd/platforms"
	refdocker "github.com/containerd/containerd/reference/docker"
	"github.com/containerd/containerd/remotes"
	"github.com/containerd/containerd/remotes/docker"
	"github.com/containerd/containerd/remotes/docker/config"
	dockercli "github.com/docker/docker/client"
	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/event"
	"github.com/goodrain/rainbond/util/criutil"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	"os"
	"text/tabwriter"
	"time"
)

type containerdImageCliFactory struct{}

func (f containerdImageCliFactory) NewClient(endpoint string, timeout time.Duration) (ImageClient, error) {
	var (
		containerdCli *containerd.Client
		imageClient   runtimeapi.ImageServiceClient
		grpcConn      *grpc.ClientConn
		err           error
	)
	imageClient, grpcConn, err = criutil.GetImageClient(context.Background(), endpoint, time.Second*3)
	if err != nil {
		return nil, err
	}
	if os.Getenv("CONTAINERD_SOCK") != "" {
		endpoint = os.Getenv("CONTAINERD_SOCK")
	}
	containerdCli, err = containerd.New(endpoint, containerd.WithTimeout(timeout))
	if err != nil {
		return nil, err
	}
	return &containerdImageCliImpl{
		client:      containerdCli,
		conn:        grpcConn,
		imageClient: imageClient,
	}, nil
}

type containerdImageCliImpl struct {
	client      *containerd.Client
	conn        *grpc.ClientConn
	imageClient runtimeapi.ImageServiceClient
}

func (c *containerdImageCliImpl) CheckIfImageExists(imageName string) (imageRef string, exists bool, err error) {
	named, err := refdocker.ParseDockerRef(imageName)
	if err != nil {
		return "", false, fmt.Errorf("parse image %s: %v", imageName, err)
	}
	imageFullName := named.String()
	ctx := namespaces.WithNamespace(context.Background(), Namespace)
	containers, err := c.imageClient.ListImages(ctx, &runtimeapi.ListImagesRequest{})
	if err != nil {
		return imageFullName, false, err
	}
	if len(containers.GetImages()) > 0 {
		for _, image := range containers.GetImages() {
			for _, repoTag := range image.GetRepoTags() {
				if repoTag == imageFullName {
					return imageFullName, true, nil
				}
			}
		}
	}
	return imageFullName, false, nil
}

func (c *containerdImageCliImpl) GetContainerdClient() *containerd.Client {
	return c.client
}

func (c *containerdImageCliImpl) GetDockerClient() *dockercli.Client {
	return nil
}

func (c *containerdImageCliImpl) ImagePull(image string, username, password string, logger event.Logger, timeout int) (*ocispec.ImageConfig, error) {
	printLog(logger, "info", fmt.Sprintf("start get image:%s", image), map[string]string{"step": "pullimage"})
	named, err := refdocker.ParseDockerRef(image)
	if err != nil {
		return nil, err
	}
	reference := named.String()
	ongoing := ctrcontent.NewJobs(reference)
	ctx := namespaces.WithNamespace(context.Background(), Namespace)
	pctx, stopProgress := context.WithCancel(ctx)
	progress := make(chan struct{})

	writer := logger.GetWriter("builder", "info")
	go func() {
		ctrcontent.ShowProgress(pctx, ongoing, c.client.ContentStore(), writer)
		close(progress)
	}()
	h := images.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
		if desc.MediaType != images.MediaTypeDockerSchema1Manifest {
			ongoing.Add(desc)
		}
		return nil, nil
	})
	defaultTLS := &tls.Config{
		InsecureSkipVerify: true,
	}
	hostOpt := config.HostOptions{}
	hostOpt.DefaultTLS = defaultTLS
	hostOpt.Credentials = func(host string) (string, string, error) {
		return username, password, nil
	}
	Tracker := docker.NewInMemoryTracker()
	options := docker.ResolverOptions{
		Tracker: Tracker,
		Hosts:   config.ConfigureHosts(pctx, hostOpt),
	}

	platformMC := platforms.Ordered([]ocispec.Platform{platforms.DefaultSpec()}...)
	opts := []containerd.RemoteOpt{
		containerd.WithImageHandler(h),
		//nolint:staticcheck
		containerd.WithSchema1Conversion, //lint:ignore SA1019 nerdctl should support schema1 as well.
		containerd.WithPlatformMatcher(platformMC),
		containerd.WithResolver(docker.NewResolver(options)),
	}
	var img containerd.Image
	img, err = c.client.Pull(pctx, reference, opts...)
	stopProgress()
	if err != nil {
		return nil, err
	}
	<-progress
	printLog(logger, "info", fmt.Sprintf("Success Pull Image：%s", reference), map[string]string{"step": "pullimage"})
	return getImageConfig(ctx, img)
}

func getImageConfig(ctx context.Context, image containerd.Image) (*ocispec.ImageConfig, error) {
	desc, err := image.Config(ctx)
	if err != nil {
		return nil, err
	}
	switch desc.MediaType {
	case ocispec.MediaTypeImageConfig, images.MediaTypeDockerSchema2Config:
		var ocispecImage ocispec.Image
		b, err := content.ReadBlob(ctx, image.ContentStore(), desc)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(b, &ocispecImage); err != nil {
			return nil, err
		}
		return &ocispecImage.Config, nil
	default:
		return nil, fmt.Errorf("unknown media type %q", desc.MediaType)
	}
}

func (c *containerdImageCliImpl) ImagePush(image, user, pass string, logger event.Logger, timeout int) error {
	printLog(logger, "info", fmt.Sprintf("start push image：%s", image), map[string]string{"step": "pushimage"})
	named, err := refdocker.ParseDockerRef(image)
	if err != nil {
		return err
	}
	reference := named.String()
	ctx := namespaces.WithNamespace(context.Background(), Namespace)
	img, err := c.client.ImageService().Get(ctx, reference)
	if err != nil {
		return errors.Wrap(err, "unable to resolve image to manifest")
	}
	desc := img.Target
	cs := c.client.ContentStore()
	if manifests, err := images.Children(ctx, cs, desc); err == nil && len(manifests) > 0 {
		matcher := platforms.NewMatcher(platforms.DefaultSpec())
		for _, manifest := range manifests {
			if manifest.Platform != nil && matcher.Match(*manifest.Platform) {
				if _, err := images.Children(ctx, cs, manifest); err != nil {
					return errors.Wrap(err, "no matching manifest")
				}
				desc = manifest
				break
			}
		}
	}
	NewTracker := docker.NewInMemoryTracker()
	options := docker.ResolverOptions{
		Tracker: NewTracker,
	}
	hostOptions := config.HostOptions{
		DefaultTLS: &tls.Config{
			InsecureSkipVerify: true,
		},
	}
	hostOptions.Credentials = func(host string) (string, string, error) {
		return user, pass, nil
	}
	options.Hosts = config.ConfigureHosts(ctx, hostOptions)
	resolver := docker.NewResolver(options)
	ongoing := newPushJobs(NewTracker)

	eg, ctx := errgroup.WithContext(ctx)
	// used to notify the progress writer
	doneCh := make(chan struct{})
	eg.Go(func() error {
		defer close(doneCh)
		jobHandler := images.HandlerFunc(func(ctx context.Context, desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {
			ongoing.add(remotes.MakeRefKey(ctx, desc))
			return nil, nil
		})

		ropts := []containerd.RemoteOpt{
			containerd.WithResolver(resolver),
			containerd.WithImageHandler(jobHandler),
		}
		return c.client.Push(ctx, reference, desc, ropts...)
	})
	writer := logger.GetWriter("builder", "info")
	eg.Go(func() error {
		var (
			ticker = time.NewTicker(100 * time.Millisecond)
			fw     = progress.NewWriter(writer)
			start  = time.Now()
			done   bool
		)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				fw.Flush()
				tw := tabwriter.NewWriter(fw, 1, 8, 1, ' ', 0)
				ctrcontent.Display(tw, ongoing.status(), start)
				tw.Flush()
				if done {
					fw.Flush()
					return nil
				}
			case <-doneCh:
				done = true
			case <-ctx.Done():
				done = true // allow ui to update once more
			}
		}
	})
	// create a container
	printLog(logger, "info", fmt.Sprintf("success push image：%s", reference), map[string]string{"step": "pushimage"})
	return nil
}

//ImageTag change docker image tag
func (c *containerdImageCliImpl) ImageTag(source, target string, logger event.Logger, timeout int) error {
	srcNamed, err := refdocker.ParseDockerRef(source)
	if err != nil {
		return err
	}
	srcImage := srcNamed.String()
	targetNamed, err := refdocker.ParseDockerRef(target)
	if err != nil {
		return err
	}
	targetImage := targetNamed.String()
	logrus.Infof(fmt.Sprintf("change image tag：%s -> %s", srcImage, targetImage))
	printLog(logger, "info", fmt.Sprintf("change image tag：%s -> %s", source, target), map[string]string{"step": "changetag"})
	ctx := namespaces.WithNamespace(context.Background(), Namespace)
	imageService := c.client.ImageService()
	image, err := imageService.Get(ctx, srcImage)
	if err != nil {
		logrus.Errorf("imagetag imageService Get error: %s", err.Error())
		return err
	}
	image.Name = targetImage
	if _, err = imageService.Create(ctx, image); err != nil {
		if errdefs.IsAlreadyExists(err) {
			if err = imageService.Delete(ctx, image.Name); err != nil {
				logrus.Errorf("imagetag imageService Delete error: %s", err.Error())
				return err
			}
			if _, err = imageService.Create(ctx, image); err != nil {
				logrus.Errorf("imageService Create error: %s", err.Error())
				return err
			}
		} else {
			logrus.Errorf("imagetag imageService Create error: %s", err.Error())
			return err
		}
	}
	logrus.Info("change image tag success")
	printLog(logger, "info", "change image tag success", map[string]string{"step": "changetag"})
	return nil
}

// ImagesPullAndPush Used to process mirroring of non local components, example: builder, runner, /rbd-mesh-data-panel
func (c *containerdImageCliImpl) ImagesPullAndPush(sourceImage, targetImage, username, password string, logger event.Logger) error {
	sourceImage, exists, err := c.CheckIfImageExists(sourceImage)
	if err != nil {
		logrus.Errorf("failed to check whether the builder mirror exists: %s", err.Error())
		return err
	}
	logrus.Infof("source image %v, targetImage %v, exists %v", sourceImage, targetImage, exists)
	if !exists {
		hubUser, hubPass := builder.GetImageUserInfoV2(sourceImage, username, password)
		if _, err := c.ImagePull(targetImage, hubUser, hubPass, logger, 15); err != nil {
			printLog(logger, "error", fmt.Sprintf("pull image %s failed %v", targetImage, err), map[string]string{"step": "builder-exector", "status": "failure"})
			return err
		}
		if err := c.ImageTag(targetImage, sourceImage, logger, 15); err != nil {
			printLog(logger, "error", fmt.Sprintf("change image tag %s to %s failed", targetImage, sourceImage), map[string]string{"step": "builder-exector", "status": "failure"})
			return err
		}
		if err := c.ImagePush(sourceImage, hubUser, hubPass, logger, 15); err != nil {
			printLog(logger, "error", fmt.Sprintf("push image %s failed %v", sourceImage, err), map[string]string{"step": "builder-exector", "status": "failure"})
			return err
		}
	}
	return nil
}

//ImageRemove remove image
func (c *containerdImageCliImpl) ImageRemove(image string) error {
	named, err := refdocker.ParseDockerRef(image)
	if err != nil {
		return err
	}
	reference := named.String()
	ctx := namespaces.WithNamespace(context.Background(), Namespace)
	imageStore := c.client.ImageService()
	err = imageStore.Delete(ctx, reference)
	if err != nil {
		logrus.Errorf("image remove ")
	}
	return err
}

//ImageSave save image to tar file
// destination destination file name eg. /tmp/xxx.tar
func (c *containerdImageCliImpl) ImageSave(image, destination string) error {
	named, err := refdocker.ParseDockerRef(image)
	if err != nil {
		return err
	}
	ctx := namespaces.WithNamespace(context.Background(), Namespace)
	var exportOpts = []archive.ExportOpt{archive.WithImage(c.client.ImageService(), named.String())}
	w, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer w.Close()
	return c.client.Export(ctx, w, exportOpts...)
}

//TrustedImagePush push image to trusted registry
func (c *containerdImageCliImpl) TrustedImagePush(image, user, pass string, logger event.Logger, timeout int) error {
	if err := CheckTrustedRepositories(image, user, pass); err != nil {
		return err
	}
	return c.ImagePush(image, user, pass, logger, timeout)
}

//ImageLoad load image from  tar file
// destination destination file name eg. /tmp/xxx.tar
func (c *containerdImageCliImpl) ImageLoad(tarFile string, logger event.Logger) error {
	ctx := namespaces.WithNamespace(context.Background(), Namespace)
	reader, err := os.OpenFile(tarFile, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	defer reader.Close()
	if _, err = c.client.Import(ctx, reader); err != nil {
		return err
	}
	return nil
}
