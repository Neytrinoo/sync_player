package main

import (
	grpc_agent "github.com/Neytrinoo/sync_player/agents"
	"github.com/Neytrinoo/sync_player/auth/proto"
	"github.com/Neytrinoo/sync_player/delivery/http"
	middleware2 "github.com/Neytrinoo/sync_player/delivery/middleware"
	"github.com/Neytrinoo/sync_player/repository/redis"
	"github.com/Neytrinoo/sync_player/usecase"
	"github.com/labstack/echo/v4"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"os"
)

func main() {
	e := echo.New()
	userSyncPlayerRepo := redis.NewUserSyncElemsRepo(os.Getenv("REDIS_ADDR"))
	useCase := usecase.NewUserSyncPlayerUseCase(userSyncPlayerRepo)
	handler := http.NewHandler(useCase, os.Getenv("REDIS_ADDR"))

	grpcConn, _ := grpc.Dial(os.Getenv("AUTH_GRPC_ADDR"), grpc.WithTransportCredentials(insecure.NewCredentials()))
	authClient := proto.NewAuthorizationClient(grpcConn)
	authAgent := grpc_agent.NewAuthGRPCAgent(authClient)

	middleware := middleware2.InitMiddleware(authAgent)

	e.GET("/", handler.PlayerStateLoop, middleware.Auth)

	e.Start("0.0.0.0:5000")
}
