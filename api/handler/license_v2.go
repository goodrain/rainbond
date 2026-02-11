package handler

import (
	"context"
	"fmt"
	"time"

	"github.com/goodrain/rainbond/api/util/license"
	"github.com/goodrain/rainbond/pkg/component/k8s"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	licenseConfigMapNamespace = "rbd-system"
	licenseConfigMapName      = "rbd-license-info"
	licenseConfigMapKey       = "license"
)

// LicenseV2Handler handles license V2 operations.
type LicenseV2Handler interface {
	GetClusterID(ctx context.Context) (string, error)
	ActivateLicense(ctx context.Context, licenseCode string, enterpriseID string) (*license.LicenseStatus, error)
	GetLicenseStatus(ctx context.Context) (*license.LicenseStatus, error)
}

type licenseV2Action struct{}

var defaultLicenseV2Handler LicenseV2Handler

// CreateLicenseV2Handler creates a singleton LicenseV2Handler.
func CreateLicenseV2Handler() {
	defaultLicenseV2Handler = &licenseV2Action{}
}

// GetLicenseV2Handler returns the singleton LicenseV2Handler.
func GetLicenseV2Handler() LicenseV2Handler {
	return defaultLicenseV2Handler
}

func (l *licenseV2Action) GetClusterID(ctx context.Context) (string, error) {
	ns, err := k8s.Default().Clientset.CoreV1().Namespaces().Get(ctx, "kube-system", metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get kube-system namespace: %w", err)
	}
	return string(ns.UID), nil
}

func (l *licenseV2Action) ActivateLicense(ctx context.Context, licenseCode string, enterpriseID string) (*license.LicenseStatus, error) {
	// Decode
	token, err := license.DecodeLicense(licenseCode)
	if err != nil {
		return license.TokenToStatus(nil, false, fmt.Sprintf("decode license: %v", err)), nil
	}

	// Get embedded public key
	pubKey, err := license.GetEmbeddedPublicKey()
	if err != nil {
		return nil, fmt.Errorf("get embedded public key: %w", err)
	}

	// Verify signature
	if err := license.VerifySignature(token, pubKey); err != nil {
		return license.TokenToStatus(token, false, fmt.Sprintf("invalid signature: %v", err)), nil
	}

	// Verify enterprise_id (strong check)
	if token.EnterpriseID != enterpriseID {
		return license.TokenToStatus(token, false, fmt.Sprintf("enterprise ID mismatch: license=%s, actual=%s", token.EnterpriseID, enterpriseID)), nil
	}

	// Verify time window
	now := time.Now()
	if now.Unix() < token.StartAt {
		return license.TokenToStatus(token, false, "license not yet valid"), nil
	}
	if now.Unix() > token.ExpireAt {
		return license.TokenToStatus(token, false, "license expired"), nil
	}

	// Record current cluster_id for audit (not validated)
	clusterID, err := l.GetClusterID(ctx)
	if err != nil {
		logrus.Warnf("Failed to get cluster ID for audit: %v", err)
	} else {
		token.ClusterID = clusterID
	}

	// Write JSON (not base64) to ConfigMap so plugins can directly json.Unmarshal
	tokenJSON, err := license.MarshalLicenseJSON(token)
	if err != nil {
		return nil, fmt.Errorf("marshal license JSON: %w", err)
	}
	if err := l.writeLicenseConfigMap(ctx, tokenJSON); err != nil {
		return nil, fmt.Errorf("write license configmap: %w", err)
	}

	// Invalidate middleware cache
	InvalidateLicenseCacheFunc()

	logrus.Infof("License activated: code=%s, company=%s, enterprise=%s, cluster=%s", token.Code, token.Company, token.EnterpriseID, token.ClusterID)
	return license.TokenToStatus(token, true, ""), nil
}

func (l *licenseV2Action) GetLicenseStatus(ctx context.Context) (*license.LicenseStatus, error) {
	// Read from ConfigMap
	licenseData, err := l.readLicenseConfigMap(ctx)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return license.TokenToStatus(nil, false, "no license configured"), nil
		}
		return nil, fmt.Errorf("read license configmap: %w", err)
	}
	if licenseData == "" {
		return license.TokenToStatus(nil, false, "no license configured"), nil
	}

	// Parse JSON directly (ConfigMap stores plain JSON, not base64)
	token, err := license.ParseLicenseJSON(licenseData)
	if err != nil {
		return license.TokenToStatus(nil, false, fmt.Sprintf("decode license: %v", err)), nil
	}

	// Get public key
	pubKey, err := license.GetEmbeddedPublicKey()
	if err != nil {
		return nil, fmt.Errorf("get embedded public key: %w", err)
	}

	// Verify signature
	if err := license.VerifySignature(token, pubKey); err != nil {
		return license.TokenToStatus(token, false, fmt.Sprintf("invalid signature: %v", err)), nil
	}

	// Check time
	now := time.Now().Unix()
	if now < token.StartAt {
		return license.TokenToStatus(token, false, "license not yet valid"), nil
	}
	if now > token.ExpireAt {
		return license.TokenToStatus(token, false, "license expired"), nil
	}

	return license.TokenToStatus(token, true, ""), nil
}

func (l *licenseV2Action) writeLicenseConfigMap(ctx context.Context, licenseData string) error {
	client := k8s.Default().Clientset.CoreV1().ConfigMaps(licenseConfigMapNamespace)

	cm, err := client.Get(ctx, licenseConfigMapName, metav1.GetOptions{})
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return err
		}
		// Create new ConfigMap
		cm = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      licenseConfigMapName,
				Namespace: licenseConfigMapNamespace,
			},
			Data: map[string]string{
				licenseConfigMapKey: licenseData,
			},
		}
		_, err = client.Create(ctx, cm, metav1.CreateOptions{})
		return err
	}

	// Update existing
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data[licenseConfigMapKey] = licenseData
	_, err = client.Update(ctx, cm, metav1.UpdateOptions{})
	return err
}

func (l *licenseV2Action) readLicenseConfigMap(ctx context.Context) (string, error) {
	cm, err := k8s.Default().Clientset.CoreV1().ConfigMaps(licenseConfigMapNamespace).Get(ctx, licenseConfigMapName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return cm.Data[licenseConfigMapKey], nil
}

// InvalidateLicenseCacheFunc is set by the middleware package to allow cache invalidation.
// This avoids an import cycle between handler and middleware.
var InvalidateLicenseCacheFunc = func() {}
