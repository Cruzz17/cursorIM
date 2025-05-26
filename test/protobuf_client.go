package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"cursorIM/internal/protocol"

	"google.golang.org/protobuf/proto"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run protobuf_client.go <token>")
		os.Exit(1)
	}

	token := os.Args[1]

	// 连接到服务器
	conn, err := net.Dial("tcp", "localhost:8083")
	if err != nil {
		log.Fatalf("连接失败: %v", err)
	}
	defer conn.Close()

	log.Println("已连接到服务器")

	// 发送认证信息
	authMsg := fmt.Sprintf("AUTH %s\n", token)
	if _, err := conn.Write([]byte(authMsg)); err != nil {
		log.Fatalf("发送认证信息失败: %v", err)
	}

	// 读取认证响应
	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	if err != nil {
		log.Fatalf("读取认证响应失败: %v", err)
	}

	response = strings.TrimSpace(response)
	if response != "OK" {
		log.Fatalf("认证失败: %s", response)
	}

	log.Println("认证成功")

	// 启动消息接收协程
	go receiveMessages(conn)

	// 发送测试消息
	sendTestMessages(conn)

	// 保持连接
	select {}
}

func sendTestMessages(conn net.Conn) {
	adapter := protocol.NewMessageAdapter()

	// 创建测试消息
	messages := []*protocol.Message{
		{
			Type:      "ping",
			ID:        "ping-1",
			SenderID:  "test-user",
			Timestamp: time.Now().Unix(),
		},
		{
			Type:           "message",
			ID:             "msg-1",
			SenderID:       "test-user",
			RecipientID:    "user2",
			Content:        "Hello from Protobuf client!",
			ConversationID: "conv-1",
			Timestamp:      time.Now().Unix(),
		},
		{
			Type:           "message",
			ID:             "msg-2",
			SenderID:       "test-user",
			RecipientID:    "user3",
			Content:        "This is a Protobuf message",
			ConversationID: "conv-2",
			Timestamp:      time.Now().Unix(),
		},
	}

	for i, msg := range messages {
		log.Printf("发送消息 %d: %s", i+1, msg.Type)

		// 转换为 Protobuf
		pbMsg, err := adapter.JSONToProtobuf(msg)
		if err != nil {
			log.Printf("转换为 Protobuf 失败: %v", err)
			continue
		}

		// 序列化
		data, err := proto.Marshal(pbMsg)
		if err != nil {
			log.Printf("序列化失败: %v", err)
			continue
		}

		// 发送消息：协议标识符(1字节) + 长度(4字节) + 数据
		if err := sendProtobufMessage(conn, data); err != nil {
			log.Printf("发送消息失败: %v", err)
			continue
		}

		log.Printf("✅ 消息 %d 发送成功", i+1)
		time.Sleep(2 * time.Second)
	}
}

func sendProtobufMessage(conn net.Conn, data []byte) error {
	writer := bufio.NewWriter(conn)

	// 写入协议标识符（0x02 表示 Protobuf）
	if err := writer.WriteByte(0x02); err != nil {
		return fmt.Errorf("写入协议标识符失败: %w", err)
	}

	// 写入消息长度
	msgLen := uint32(len(data))
	if err := binary.Write(writer, binary.BigEndian, msgLen); err != nil {
		return fmt.Errorf("写入消息长度失败: %w", err)
	}

	// 写入消息数据
	if _, err := writer.Write(data); err != nil {
		return fmt.Errorf("写入消息数据失败: %w", err)
	}

	// 刷新缓冲区
	return writer.Flush()
}

func receiveMessages(conn net.Conn) {
	adapter := protocol.NewMessageAdapter()
	reader := bufio.NewReader(conn)

	for {
		// 读取协议标识符
		protocolFlag, err := reader.ReadByte()
		if err != nil {
			log.Printf("读取协议标识符失败: %v", err)
			return
		}

		// 读取消息长度
		var msgLen uint32
		if err := binary.Read(reader, binary.BigEndian, &msgLen); err != nil {
			log.Printf("读取消息长度失败: %v", err)
			return
		}

		// 读取消息数据
		data := make([]byte, msgLen)
		if _, err := reader.Read(data); err != nil {
			log.Printf("读取消息数据失败: %v", err)
			return
		}

		// 根据协议标识符解析消息
		var protocolType protocol.ProtocolType
		switch protocolFlag {
		case 0x01:
			protocolType = protocol.ProtocolTypeJSON
		case 0x02:
			protocolType = protocol.ProtocolTypeProtobuf
		default:
			log.Printf("未知的协议标识符: 0x%02x", protocolFlag)
			continue
		}

		// 反序列化消息
		message, err := adapter.DeserializeMessage(data, protocolType)
		if err != nil {
			log.Printf("反序列化消息失败: %v", err)
			continue
		}

		log.Printf("📨 收到消息 (协议: %s): Type=%s, From=%s, Content=%s",
			protocolType, message.Type, message.SenderID, message.Content)
	}
}
