package criutil

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
	"k8s.io/kubernetes/pkg/kubelet/util"
	"time"
)

const (
	defaultTimeout = 2 * time.Second
	// use same message size as cri remote client in kubelet.
	maxMsgSize = 1024 * 1024 * 16
)

var RuntimeEndpoint string
var defaultRuntimeEndpoints = []string{"unix:///var/run/dockershim.sock", "unix:///run/docker/containerd/containerd.sock", "unix:///run/containerd/containerd.sock", "unix:///run/crio/crio.sock", "unix:///var/run/cri-dockerd.sock"}

func getConnection(endPoints []string, timeout time.Duration) (*grpc.ClientConn, error) {
	if endPoints == nil || len(endPoints) == 0 {
		return nil, fmt.Errorf("endpoint is not set")
	}
	endPointsLen := len(endPoints)
	var conn *grpc.ClientConn
	for indx, endPoint := range endPoints {
		logrus.Debugf("connect using endpoint '%s' with '%s' timeout", endPoint, timeout)
		addr, dialer, err := util.GetAddressAndDialer(endPoint)
		if err != nil {
			if indx == endPointsLen-1 {
				return nil, err
			}
			logrus.Error(err)
			continue
		}
		conn, err = grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(timeout), grpc.WithContextDialer(dialer))
		if err != nil {
			errMsg := errors.Wrapf(err, "connect endpoint '%s', make sure you are running as root and the endpoint has been started", endPoint)
			if indx == endPointsLen-1 {
				return nil, errMsg
			}
			logrus.Error(errMsg)
		} else {
			logrus.Debugf("connected successfully using endpoint: %s", endPoint)
			break
		}
	}
	return conn, nil
}

func GetRuntimeClient(ctx context.Context, endpoint string, timeout time.Duration) (v1alpha2.RuntimeServiceClient, *grpc.ClientConn, error) {
	// Set up a connection to the server.
	conn, err := getRuntimeClientConnection(ctx, endpoint, timeout)
	if err != nil {
		return nil, nil, errors.Wrap(err, "connect")
	}
	runtimeClient := v1alpha2.NewRuntimeServiceClient(conn)
	return runtimeClient, conn, nil
}

func getRuntimeClientConnection(ctx context.Context, endpoint string, timeout time.Duration) (*grpc.ClientConn, error) {
	return getConnection([]string{endpoint}, timeout)
}

func GetImageClient(ctx context.Context, endpoint string, timeout time.Duration) (v1alpha2.ImageServiceClient, *grpc.ClientConn, error) {
	// Set up a connection to the server.
	conn, err := getRuntimeClientConnection(ctx, endpoint, timeout)
	if err != nil {
		return nil, nil, errors.Wrap(err, "connect")
	}
	runtimeClient := v1alpha2.NewImageServiceClient(conn)
	return runtimeClient, conn, nil
}
