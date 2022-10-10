package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/containerd/containerd"
	"github.com/docker/distribution/reference"
	dtypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	dockercli "github.com/docker/docker/client"
	"github.com/goodrain/rainbond/builder"
	"github.com/goodrain/rainbond/event"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	"io"
	"os"
	"strings"
	"time"
)

type dockerImageCliFactory struct{}

func (f dockerImageCliFactory) NewClient(endpoint string, timeout time.Duration) (ImageClient, error) {
	if os.Getenv("DOCKER_API_VERSION") == "" {
		os.Setenv("DOCKER_API_VERSION", "1.40")
	}
	cli, err := dockercli.NewClientWithOpts(dockercli.FromEnv)
	if err != nil {
		return nil, err
	}
	return &dockerImageCliImpl{
		client: cli,
	}, nil
}

type dockerImageCliImpl struct {
	client *dockercli.Client
}

func (d *dockerImageCliImpl) CheckIfImageExists(imageName string) (imageRef string, exists bool, err error) {
	repo, err := reference.Parse(imageName)
	if err != nil {
		return "", false, fmt.Errorf("parse image %s: %v", imageName, err)
	}
	named := repo.(reference.Named)
	tag := "latest"
	if t, ok := repo.(reference.Tagged); ok {
		tag = t.Tag()
	}
	imageFullName := named.Name() + ":" + tag

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	imageSummarys, err := d.client.ImageList(ctx, dtypes.ImageListOptions{
		Filters: filters.NewArgs(filters.KeyValuePair{Key: "reference", Value: imageFullName}),
	})
	if err != nil {
		return "", false, fmt.Errorf("list images: %v", err)
	}
	for _, imageSummary := range imageSummarys {
		fmt.Printf("%#v", imageSummary.RepoTags)
	}

	_ = imageSummarys

	return imageFullName, len(imageSummarys) > 0, nil
}

func (d *dockerImageCliImpl) GetContainerdClient() *containerd.Client {
	return nil
}

func (d *dockerImageCliImpl) GetDockerClient() *dockercli.Client {
	return d.client
}

func (d *dockerImageCliImpl) ImagePull(image string, username, password string, logger event.Logger, timeout int) (*ocispec.ImageConfig, error) {
	printLog(logger, "info", fmt.Sprintf("start get image:%s", image), map[string]string{"step": "pullimage"})
	var pullipo = dtypes.ImagePullOptions{}
	if username != "" && password != "" {
		auth, err := EncodeAuthToBase64(dtypes.AuthConfig{Username: username, Password: password})
		if err != nil {
			logrus.Errorf("make auth base63 push image error: %s", err.Error())
			printLog(logger, "error", fmt.Sprintf("Failed to generate a Token to get the image"), map[string]string{"step": "builder-exector", "status": "failure"})
			return nil, err
		}
		pullipo.RegistryAuth = auth
	}
	rf, err := reference.ParseAnyReference(image)
	if err != nil {
		logrus.Errorf("reference image error: %s", err.Error())
		return nil, err
	}
	//最少一分钟
	if timeout < 1 {
		timeout = 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*time.Duration(timeout))
	defer cancel()
	//TODO: 使用1.12版本api的bug “repository name must be canonical”，使用rf.String()完整的镜像地址
	readcloser, err := d.client.ImagePull(ctx, rf.String(), pullipo)
	if err != nil {
		logrus.Debugf("image name: %s readcloser error: %v", image, err.Error())
		if strings.HasSuffix(err.Error(), "does not exist or no pull access") {
			printLog(logger, "error", fmt.Sprintf("image: %s does not exist or is not available", image), map[string]string{"step": "pullimage", "status": "failure"})
			return nil, fmt.Errorf("Image(%s) does not exist or no pull access", image)
		}
		return nil, err
	}
	defer readcloser.Close()
	dec := json.NewDecoder(readcloser)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		var jm JSONMessage
		if err := dec.Decode(&jm); err != nil {
			if err == io.EOF {
				break
			}
			logrus.Debugf("error decoding jm(JSONMessage): %v", err)
			return nil, err
		}
		if jm.Error != nil {
			logrus.Debugf("error pulling image: %v", jm.Error)
			return nil, jm.Error
		}
		printLog(logger, "debug", fmt.Sprintf(jm.JSONString()), map[string]string{"step": "progress"})
		logrus.Debug(jm.JSONString())
	}
	printLog(logger, "debug", "Get the image information and its raw representation", map[string]string{"step": "progress"})
	ins, _, err := d.client.ImageInspectWithRaw(ctx, image)
	if err != nil {
		printLog(logger, "debug", "Fail to get the image information and its raw representation", map[string]string{"step": "progress"})
		return nil, err
	}
	printLog(logger, "info", fmt.Sprintf("Success Pull Image：%s", image), map[string]string{"step": "pullimage"})
	exportPorts := make(map[string]struct{})
	for port := range ins.Config.ExposedPorts {
		exportPorts[string(port)] = struct{}{}
	}
	return &ocispec.ImageConfig{
		User:         ins.Config.User,
		ExposedPorts: exportPorts,
		Env:          ins.Config.Env,
		Entrypoint:   ins.Config.Entrypoint,
		Cmd:          ins.Config.Cmd,
		Volumes:      ins.Config.Volumes,
		WorkingDir:   ins.Config.WorkingDir,
		Labels:       ins.Config.Labels,
		StopSignal:   ins.Config.StopSignal,
	}, nil
}

func (d *dockerImageCliImpl) ImagePush(image, user, pass string, logger event.Logger, timeout int) error {
	printLog(logger, "info", fmt.Sprintf("start push image：%s", image), map[string]string{"step": "pushimage"})
	if timeout < 1 {
		timeout = 1
	}
	if user == "" {
		user = os.Getenv("LOCAL_HUB_USER")
	}
	if pass == "" {
		pass = os.Getenv("LOCAL_HUB_PASS")
	}
	ref, err := reference.ParseNormalizedNamed(image)
	if err != nil {
		return err
	}
	var opts dtypes.ImagePushOptions
	pushauth, err := EncodeAuthToBase64(dtypes.AuthConfig{
		Username:      user,
		Password:      pass,
		ServerAddress: reference.Domain(ref),
	})
	if err != nil {
		logrus.Errorf("make auth base63 push image error: %s", err.Error())
		if logger != nil {
			logger.Error(fmt.Sprintf("Failed to generate a token to get the image"), map[string]string{"step": "builder-exector", "status": "failure"})
		}
		return err
	}
	opts.RegistryAuth = pushauth
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*time.Duration(timeout))
	defer cancel()
	readcloser, err := d.client.ImagePush(ctx, image, opts)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			printLog(logger, "error", fmt.Sprintf("image %s does not exist, cannot be pushed", image), map[string]string{"step": "pushimage", "status": "failure"})
			return fmt.Errorf("Image(%s) does not exist", image)
		}
		return err
	}
	if readcloser != nil {
		defer readcloser.Close()
		dec := json.NewDecoder(readcloser)
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			var jm JSONMessage
			if err := dec.Decode(&jm); err != nil {
				if err == io.EOF {
					break
				}
				return err
			}
			if jm.Error != nil {
				return jm.Error
			}
			printLog(logger, "debug", jm.JSONString(), map[string]string{"step": "progress"})
		}
	}
	printLog(logger, "info", fmt.Sprintf("success push image：%s", image), map[string]string{"step": "pushimage"})
	return nil
}

//ImageTag change docker image tag
func (d *dockerImageCliImpl) ImageTag(source, target string, logger event.Logger, timeout int) error {
	logrus.Debugf(fmt.Sprintf("change image tag：%s -> %s", source, target))
	printLog(logger, "info", fmt.Sprintf("change image tag：%s -> %s", source, target), map[string]string{"step": "changetag"})
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*time.Duration(timeout))
	defer cancel()
	err := d.client.ImageTag(ctx, source, target)
	if err != nil {
		logrus.Debugf("image tag err: %s", err.Error())
		return err
	}
	logrus.Debugf("change image tag success")
	printLog(logger, "info", "change image tag success", map[string]string{"step": "changetag"})
	return nil
}

// ImagesPullAndPush Used to process mirroring of non local components, example: builder, runner, /rbd-mesh-data-panel
func (d *dockerImageCliImpl) ImagesPullAndPush(sourceImage, targetImage, username, password string, logger event.Logger) error {
	sourceImage, exists, err := d.CheckIfImageExists(sourceImage)
	if err != nil {
		logrus.Errorf("failed to check whether the builder mirror exists: %s", err.Error())
		return err
	}
	logrus.Debugf("source image %v, targetImage %v, exists %v", sourceImage, targetImage, exists)
	if !exists {
		hubUser, hubPass := builder.GetImageUserInfoV2(sourceImage, username, password)
		if _, err := d.ImagePull(targetImage, hubUser, hubPass, logger, 15); err != nil {
			printLog(logger, "error", fmt.Sprintf("pull image %s failed %v", targetImage, err), map[string]string{"step": "builder-exector", "status": "failure"})
			return err
		}
		if err := d.ImageTag(targetImage, sourceImage, logger, 15); err != nil {
			printLog(logger, "error", fmt.Sprintf("change image tag %s to %s failed", targetImage, sourceImage), map[string]string{"step": "builder-exector", "status": "failure"})
			return err
		}
		if err := d.ImagePush(sourceImage, hubUser, hubPass, logger, 15); err != nil {
			printLog(logger, "error", fmt.Sprintf("push image %s failed %v", sourceImage, err), map[string]string{"step": "builder-exector", "status": "failure"})
			return err
		}
	}
	return nil
}

//ImageRemove remove image
func (d *dockerImageCliImpl) ImageRemove(image string) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	_, err := d.client.ImageRemove(ctx, image, dtypes.ImageRemoveOptions{Force: true})
	return err
}

//ImageSave save image to tar file
// destination destination file name eg. /tmp/xxx.tar
func (d *dockerImageCliImpl) ImageSave(image, destination string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rc, err := d.client.ImageSave(ctx, []string{image})
	if err != nil {
		return err
	}
	defer rc.Close()
	return CopyToFile(destination, rc)
}

//TrustedImagePush push image to trusted registry
func (d *dockerImageCliImpl) TrustedImagePush(image, user, pass string, logger event.Logger, timeout int) error {
	if err := CheckTrustedRepositories(image, user, pass); err != nil {
		return err
	}
	return d.ImagePush(image, user, pass, logger, timeout)
}

//ImageLoad load image from  tar file
// destination destination file name eg. /tmp/xxx.tar
func (d *dockerImageCliImpl) ImageLoad(tarFile string, logger event.Logger) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reader, err := os.OpenFile(tarFile, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	defer reader.Close()

	rc, err := d.client.ImageLoad(ctx, reader, false)
	if err != nil {
		return err
	}
	if rc.Body != nil {
		defer rc.Body.Close()
		dec := json.NewDecoder(rc.Body)
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			var jm JSONMessage
			if err := dec.Decode(&jm); err != nil {
				if err == io.EOF {
					break
				}
				return err
			}
			if jm.Error != nil {
				return jm.Error
			}
			logger.Info(jm.JSONString(), map[string]string{"step": "build-progress"})
		}
	}
	return nil
}
