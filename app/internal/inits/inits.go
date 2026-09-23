package inits

import (
	"app/internal/router"
	"core/tools"

	"github.com/mszlu521/thunder/config"
	"github.com/mszlu521/thunder/database"
	"github.com/mszlu521/thunder/server"
	"github.com/mszlu521/thunder/tools/jwt"
)

func Init(s *server.Server, conf *config.Config) {
	database.InitPostgres(conf.DB.Postgres)
	database.InitRedis(conf.DB.Redis)
	jwt.Init(conf.Jwt.GetSecret())
	tools.RegisterBuiltins()
	s.RegisterRouters(
		&router.Event{},
		&router.AuthRouter{},
		&router.SubscriptionRouter{},
		&router.AgentsRouter{},
		&router.ProviderRouter{},
		&router.ToolRouter{},
	)
}
