package main

import (
	"flag"
	"github.com/Duke1616/vuefinder-go/pkg/finder"
	"github.com/Duke1616/vuefinder-go/pkg/ginx"
	"github.com/Duke1616/vuefinder-go/pkg/web"
	"github.com/gin-gonic/gin"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"log"
)

func main() {
	// 定义命令行参数
	host := flag.String("host", "127.0.0.1:22", "SSH server host and port")
	user := flag.String("user", "", "SSH username")
	password := flag.String("password", "", "SSH password")

	// 解析命令行参数
	flag.Parse()

	// 检查必填参数
	if *user == "" || *password == "" {
		log.Fatal("Username and password are required")
	}

	// 连接到 SSH 服务器
	client, err := ConnectSSH(*host, *user, *password)

	sftpClient, err := sftp.NewClient(client)
	f := finder.NewSftpFinder(sftpClient, *user)
	handler := web.NewHandler(f)
	mlds := ginx.NewMiddleware()
	engine := gin.Default()
	engine.Use(mlds...)
	handler.RegisterRoutes(engine)
	err = engine.Run(":8350")
	if err != nil {
		panic(err)
	}
}

func ConnectSSH(host, user, password string) (*ssh.Client, error) {
	// 创建 SSH 客户端配置
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 不推荐在生产环境中使用
	}

	// 连接到 SSH 服务器
	client, err := ssh.Dial("tcp", host, config)
	if err != nil {
		return nil, err
	}

	return client, nil
}
