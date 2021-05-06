// Code generated by 'freedom new-project fshop'
package main

import (
	"time"

	"github.com/8treenet/freedom"
	"github.com/8treenet/freedom/example/fshop/adapter/consumer"
	_ "github.com/8treenet/freedom/example/fshop/adapter/controller" //引入输入适配器 http路由
	_ "github.com/8treenet/freedom/example/fshop/adapter/repository" //引入输出适配器 repository资源库
	"github.com/8treenet/freedom/example/fshop/server/conf"          //需要开启 server/conf/infra/kafka.toml open = true
	"github.com/8treenet/freedom/infra/requests"
	"github.com/8treenet/freedom/middleware"
	"github.com/go-redis/redis"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	app := freedom.NewApplication()
	installDatabase(app)
	installRedis(app)
	installMiddleware(app)

	addrRunner := app.NewH2CRunner(conf.Get().App.Other["listen_addr"].(string))
	//app.InstallParty("/fshop")
	liveness(app)
	consumer.Start() //定时器
	app.Run(addrRunner, *conf.Get().App)
}

func installMiddleware(app freedom.Application) {
	//Recover中间件
	app.InstallMiddleware(middleware.NewRecover())
	//Trace链路中间件
	app.InstallMiddleware(middleware.NewTrace("x-request-id"))

	//自定义请求日志配置文件
	//1.打印UA
	//2.修改请求日志前缀
	loggerConfig := middleware.DefaultLoggerConfig()
	loggerConfig.MessageHeaderKeys = append(loggerConfig.MessageHeaderKeys, "User-Agent")
	loggerConfig.Title = "[请求日志]"

	//日志中间件，每个请求一个logger
	app.InstallMiddleware(middleware.NewRequestLogger("x-request-id", loggerConfig))
	//logRow中间件，每一行日志都会触发回调。如果返回true，将停止中间件遍历回调。
	app.Logger().Handle(middleware.DefaultLogRowHandle)

	//HttpClient 普罗米修斯中间件，监控下游的API请求。
	middle := middleware.NewClientPrometheus(conf.Get().App.Other["service_name"].(string), freedom.Prometheus())
	requests.InstallMiddleware(middle)

	//总线中间件，处理上下游透传的Header
	app.InstallBusMiddleware(middleware.NewBusFilter())
}

func installDatabase(app freedom.Application) {
	app.InstallDB(func() interface{} {
		conf := conf.Get().DB
		db, e := gorm.Open(mysql.Open(conf.Addr), &gorm.Config{})
		if e != nil {
			freedom.Logger().Fatal(e.Error())
		}

		sqlDB, err := db.DB()
		if err != nil {
			freedom.Logger().Fatal(e.Error())
		}
		sqlDB.SetMaxIdleConns(conf.MaxIdleConns)
		sqlDB.SetMaxOpenConns(conf.MaxOpenConns)
		sqlDB.SetConnMaxLifetime(time.Duration(conf.ConnMaxLifeTime) * time.Second)
		return db
	})
}

func installRedis(app freedom.Application) {
	app.InstallRedis(func() (client redis.Cmdable) {
		cfg := conf.Get().Redis
		opt := &redis.Options{
			Addr:               cfg.Addr,
			Password:           cfg.Password,
			DB:                 cfg.DB,
			MaxRetries:         cfg.MaxRetries,
			PoolSize:           cfg.PoolSize,
			ReadTimeout:        time.Duration(cfg.ReadTimeout) * time.Second,
			WriteTimeout:       time.Duration(cfg.WriteTimeout) * time.Second,
			IdleTimeout:        time.Duration(cfg.IdleTimeout) * time.Second,
			IdleCheckFrequency: time.Duration(cfg.IdleCheckFrequency) * time.Second,
			MaxConnAge:         time.Duration(cfg.MaxConnAge) * time.Second,
			PoolTimeout:        time.Duration(cfg.PoolTimeout) * time.Second,
		}
		client = redis.NewClient(opt)
		if e := client.Ping().Err(); e != nil {
			freedom.Logger().Fatal(e.Error())
		}
		return
	})
}

func liveness(app freedom.Application) {
	app.Iris().Get("/ping", func(ctx freedom.Context) {
		ctx.WriteString("pong")
	})
}
