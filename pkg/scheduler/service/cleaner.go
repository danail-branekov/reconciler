package service

import (
	"context"
	"time"

	"github.com/kyma-incubator/reconciler/pkg/scheduler/reconciliation"
	"go.uber.org/zap"
)

type CleanerConfig struct {
	PurgeEntitiesOlderThan time.Duration
	CleanerInterval        time.Duration
}

type cleaner struct {
	logger *zap.SugaredLogger
}

func newCleaner(logger *zap.SugaredLogger) *cleaner {
	return &cleaner{
		logger: logger,
	}
}

func (c *cleaner) Run(ctx context.Context, transition *ClusterStatusTransition, config *CleanerConfig) error {
	c.logger.Infof("Starting entities cleaner: interval for clearing old Reconciliation and Operation entities "+
		"is %s. Cleaner will remove entities older than %s", config.CleanerInterval.String(), config.PurgeEntitiesOlderThan.String())

	ticker := time.NewTicker(config.CleanerInterval)
	c.purgeReconciliations(transition, config) //check for entities now, otherwise first check would be trigger by ticker
	for {
		select {
		case <-ticker.C:
			c.purgeReconciliations(transition, config)
		case <-ctx.Done():
			c.logger.Info("Stopping cleaner because parent context got closed")
			ticker.Stop()
			return nil
		}
	}
}

func (c *cleaner) purgeReconciliations(transition *ClusterStatusTransition, config *CleanerConfig) {
	deadline := time.Now().UTC().Add(-1 * config.PurgeEntitiesOlderThan)
	reconciliations, err := transition.ReconciliationRepository().GetReconciliations(&reconciliation.WithCreationDateBefore{
		Time: deadline,
	})
	if err != nil {
		c.logger.Errorf("Cleaner failed to get reconciliations older than %s: %s", deadline.String(), err.Error())
	}

	for i := range reconciliations {
		c.logger.Infof("Cleaner triggered for the Reconciliation and dependent Operations with SchedulingID '%s' "+
			"(created: %s)", reconciliations[i].SchedulingID, reconciliations[i].Created)

		id := reconciliations[i].SchedulingID
		err := transition.ReconciliationRepository().RemoveReconciliation(id)
		if err != nil {
			c.logger.Errorf("Cleaner failed to remove reconciliation with schedulingID '%s': %s", id, err.Error())
		}
	}
}
