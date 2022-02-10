package clickhouse

import (
	"context"
	"fmt"
	"github.com/squids-io/clickhouse-operator/pkg/chop"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

const namespace = "squids-user"


func ResetUserParams(hostname, oldUsername string) (username, password string) {
	ctx := context.TODO()
	kubeClient, _ := chop.GetClientset("", "")
	clusterName := strings.Split(hostname, "-")[2]
	secreteName := fmt.Sprintf("clickhouse-%s-component-user-suffix", clusterName)
	secret, _ := kubeClient.CoreV1().Secrets(namespace).Get(ctx, secreteName, metav1.GetOptions{})
	password = string(secret.Data["password"])
	username = oldUsername
	if oldUsername == "" {
		username = "root"
	}
	return username, password
}
