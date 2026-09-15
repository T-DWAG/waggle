package main

import (
	"app/internal/inits"
	"github.com/mszlu521/thunder/config"
	"github.com/mszlu521/thunder/logs"
	"github.com/mszlu521/thunder/server"
)

func main() {
	//1. 加载配置  默认是 etc/config.yml
	config.Init()
	conf := config.GetConfig()
	//2. 加载日志
	logs.Init(conf.Log)
	//3. 创建服务
	s := server.NewServer(conf)
	//4. 初始化模块
	inits.Init(s, conf)
	s.Start()
}
