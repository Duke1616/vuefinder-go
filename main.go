package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Duke1616/vuefinder-go/pkg/finder"
	"github.com/Duke1616/vuefinder-go/pkg/ginx"
	"github.com/Duke1616/vuefinder-go/pkg/web"
	"github.com/gin-gonic/gin"
	"github.com/pkg/sftp"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
)

var (
	host       string
	user       string
	password   string
	privateKey string
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "vuefinder-go",
		Short: "VueFinder Go - SFTP file browser",
		Long:  "A web-based file browser for SFTP servers",
		Run:   run,
	}

	rootCmd.Flags().StringVarP(&host, "host", "H", "127.0.0.1:22", "SSH server host and port")
	rootCmd.Flags().StringVarP(&user, "user", "u", "", "SSH username (required)")
	rootCmd.Flags().StringVarP(&password, "password", "p", "", "SSH password (optional, use with --key)")
	rootCmd.Flags().StringVarP(&privateKey, "key", "k", "", "Path to private key file (optional, use with --password)")

	err := rootCmd.MarkFlagRequired("user")
	if err != nil {
		return
	}

	if err = rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func run(cmd *cobra.Command, args []string) {
	// 检查必填参数
	if user == "" {
		fmt.Println("Error: username is required")
		err := cmd.Help()
		if err != nil {
			return
		}
		return
	}

	// 检查认证方式：必须提供密码或密钥之一
	if password == "" && privateKey == "" {
		fmt.Println("Error: either password or private key must be provided")
		err := cmd.Help()
		if err != nil {
			return
		}
		return
	}

	// 连接到 SSH 服务器
	client, err := ConnectSSH(host, user, password, privateKey)
	if err != nil {
		log.Fatalf("Failed to connect to SSH server: %v", err)
	}
	defer client.Close()

	sftpClient, err := sftp.NewClient(client)
	if err != nil {
		log.Fatalf("Failed to create SFTP client: %v", err)
	}
	defer sftpClient.Close()

	f := finder.NewSftpFinder(sftpClient)
	handler := web.NewHandler()
	handler.SetFinder(20, f)
	mlds := ginx.NewMiddleware()
	engine := gin.Default()
	engine.Use(mlds...)
	handler.RegisterRoutes(engine)

	log.Printf("Starting server on :8350")
	log.Printf("Connected to SSH server: %s@%s", user, host)
	if err := engine.Run(":8350"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func ConnectSSH(host, user, password, privateKeyPath string) (*ssh.Client, error) {
	// 创建认证方法列表
	var authMethods []ssh.AuthMethod

	// 添加密码认证
	if password != "" {
		authMethods = append(authMethods, ssh.Password(password))
	}

	// 添加密钥认证
	if privateKeyPath != "" {
		key, err := loadPrivateKey(privateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(key))
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no authentication method provided")
	}

	// 创建 SSH 客户端配置
	config := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // 不推荐在生产环境中使用
		Timeout:         10 * time.Second,
	}

	// 连接到 SSH 服务器
	client, err := ssh.Dial("tcp", host, config)
	if err != nil {
		return nil, fmt.Errorf("ssh dial failed: %w", err)
	}

	return client, nil
}

func loadPrivateKey(path string) (ssh.Signer, error) {
	// 读取私钥文件
	keyBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	// 尝试解析私钥（支持加密的私钥）
	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		// 如果解析失败，可能是加密的私钥，尝试使用空密码
		signer, err = ssh.ParsePrivateKeyWithPassphrase(keyBytes, []byte{})
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w (note: encrypted keys require passphrase)", err)
		}
	}

	return signer, nil
}
