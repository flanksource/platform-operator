package k8s

import (
	"os"
	// log "github.com/sirupsen/logrus"
	// v1 "k8s.io/api/core/v1"
	// "k8s.io/apimachinery/pkg/api/errors"
	// "k8s.io/apimachinery/pkg/api/resource"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	GetKubeConfig func() (string, error)
}

func GetDefaultClient() Client {
	return Client{
		GetKubeConfig: func() (string, error) {
			kubeconfig := os.Getenv("KUBECONFIG")
			if kubeconfig != "" {
				return kubeconfig, nil
			}
			return    os.ExpandEnv("$HOME/.kube/config"), nil
		},
	}
}

// GetDynamicClient creates a new k8s client
func (c *Client) GetDynamicClient() (dynamic.Interface, error) {
	kubeconfig, err := c.GetKubeConfig()
	if err != nil {
		return nil, err
	}
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	return dynamic.NewForConfig(cfg)
}

// GetClientset creates a new k8s client
func (c *Client) GetClientset() (*kubernetes.Clientset, error) {
	kubeconfig, err := c.GetKubeConfig()
	if err != nil {
		return nil, err
	}
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfigOrDie(cfg), nil
}
