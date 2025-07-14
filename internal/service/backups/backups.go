package backups

import (
	"github.com/eduardolat/pgbackweb/internal/config"
	"github.com/eduardolat/pgbackweb/internal/cron"
	"github.com/eduardolat/pgbackweb/internal/database/dbgen"
	"github.com/eduardolat/pgbackweb/internal/service/executions"
)

type Service struct {
	env               config.Env
	dbgen             *dbgen.Queries
	cr                *cron.Cron
	executionsService *executions.Service
}

func New(
	env config.Env,
	dbgen *dbgen.Queries,
	cr *cron.Cron,
	executionsService *executions.Service,
) *Service {
	return &Service{
		env:               env,
		dbgen:             dbgen,
		cr:                cr,
		executionsService: executionsService,
	}
}
