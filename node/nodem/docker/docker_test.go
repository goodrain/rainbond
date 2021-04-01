package docker

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func Test_saveORUpdateFile(t *testing.T) {
	t.Log(saveORUpdateFile([]byte("123")))
}

func TestSync(t *testing.T) {
	config, err := clientcmd.BuildConfigFromFlags("", "/Users/fanyangyang/Documents/company/goodrain/remote/192.168.2.200/admin.kubeconfig")
	if err != nil {
		t.Fatal("build config from flag error: ", err.Error())
	}
	config.QPS = 50
	config.Burst = 100
	cli, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatal("new for config error: ", err.Error())
	}
	secretName := "rbd-docker-secret"
	namespace := "rbd-system"
	secret := &corev1.Secret{}
	secret.Name = secretName
	data := make(map[string][]byte)
	data["cert"] = []byte(`-----BEGIN CERTIFICATE-----
MIIClTCCAf4CCQCrz/TYniQE3zANBgkqhkiG9w0BAQsFADCBjTELMAkGA1UEBhMC
Q04xEDAOBgNVBAgMB0JlaWppbmcxEDAOBgNVBAcMB0JlaWppbmcxETAPBgNVBAoM
CGdvb2RyYWluMQ8wDQYDVQQLDAZzeXN0ZW0xFDASBgNVBAMMC2dvb2RyYWluLm1l
MSAwHgYJKoZIhvcNAQkBFhFyb290QGdvb2RyYWluLmNvbTAgFw0xNjA0MjYxMTE0
NTZaGA8yMTE2MDQwMjExMTQ1NlowgY0xCzAJBgNVBAYTAkNOMRAwDgYDVQQIDAdC
ZWlqaW5nMRAwDgYDVQQHDAdCZWlqaW5nMREwDwYDVQQKDAhnb29kcmFpbjEPMA0G
A1UECwwGc3lzdGVtMRQwEgYDVQQDDAtnb29kcmFpbi5tZTEgMB4GCSqGSIb3DQEJ
ARYRcm9vdEBnb29kcmFpbi5jb20wgZ8wDQYJKoZIhvcNAQEBBQADgY0AMIGJAoGB
ALSWCeeDuge8N9coS2w+7q1M9RdTI5O85E984t97yTJNOVWcxCjPZRkTSEGPXjuv
QUCqBKbXWJX++dcDE8Xrx5yGQZywNOUi4sBjxvkO0+kPH3cBcZYb6+Jt2Boyk0ja
lPPJ1n7YlIfbps+MCGoSlsozh1ms8/MmSdDhYnA2HhZhAgMBAAEwDQYJKoZIhvcN
AQELBQADgYEAcp2ETrYEvzxty5fFQXuEUdJQBjXUUaO4YuFuAHZnX0mBdLFs8JHt
Dv5SVos+Rd/zF9Szg68uBOzkrFODygyzUjPgUtP1oIrPMFgvraYmbBQNdzT/7zBN
OIBrj5fMeg27zqsV/2Qr1YuzfMZcgQG9KtPSe57RZH9kF7pCl+cqetc=
-----END CERTIFICATE-----`)
	secret.Data = data
	if _, err := cli.CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{}); err != nil {
		t.Fatalf("create secret error: %s", err.Error())
	}
	if err := SyncDockerCertFromSecret(cli, namespace, secretName); err != nil {
		t.Fatalf("sync secret error: %s", err.Error())
	}
	cli.CoreV1().Secrets(namespace).Delete(context.Background(), secretName, metav1.DeleteOptions{})
}
