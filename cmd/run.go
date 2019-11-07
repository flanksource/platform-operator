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

		client := k8s.GetDefaultClient()

		watchDuration, err := time.ParseDuration(watchInterval)
		if err != nil {
			log.Fatalf("Invalid duration: %s", watchInterval)
		}

		controllers.CleanupOperator(client, watchDuration)
		log.Infof("Done")
	},
}

func init() {
	Run.Flags().StringVar(&watchInterval, "watch-interval", "5m", "The watch interval to check for actions")
}
