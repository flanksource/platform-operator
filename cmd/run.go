package cmd

import (
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/moshloop/platform-operator/pkg/controllers"
	"github.com/moshloop/platform-operator/pkg/k8s"
)

var watchInterval string

var Run = &cobra.Command{
	Use:   "serve",
	Short: "Run all controllers",
	Run: func(cmd *cobra.Command, args []string) {

		watchDuration, err := time.ParseDuration(watchInterval)
		if err != nil {
			log.Fatalf("Invalid duration: %s", watchInterval)
		}

		client, err := k8s.GetClientSet()
		if err != nil {
			log.Fatalf("Cannot get k8s client: %v", err)
		}
		controllers.CleanupOperator(client, watchDuration)
	},
}

func init() {
	Run.Flags().StringVar(&watchInterval, "watch-interval", "5m", "The watch interval to check for actions")
}
