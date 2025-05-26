package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"cursorIM/internal/protocol"
)

func main() {
	fmt.Println("🚀 CursorIM 协议性能测试工具")
	fmt.Println("========================================")

	// 创建测试消息
	testMsg := protocol.CreateTestMessage()

	// 运行性能测试
	iterations := 10000
	fmt.Printf("测试迭代次数: %d\n\n", iterations)

	results := protocol.BenchmarkEncoders(testMsg, iterations)

	// 输出结果表格
	printBenchmarkResults(results)

	// 输出详细的大小对比
	printSizeComparison(testMsg, results)

	// 输出选择建议
	printRecommendations()
}

func printBenchmarkResults(results map[protocol.EncodingType]*protocol.BenchmarkResult) {
	fmt.Printf("%-15s %-12s %-12s %-10s %-15s\n", "协议类型", "编码时间", "解码时间", "大小(字节)", "压缩比")
	fmt.Println("-------------------------------------------------------------------------")

	// 按性能排序显示
	order := []protocol.EncodingType{
		protocol.EncodingJSON,
		// protocol.EncodingMessagePack, // 暂时注释，需要添加依赖
		// protocol.EncodingProtobuf,    // 暂时注释，需要添加依赖
	}

	for _, encodingType := range order {
		if result, ok := results[encodingType]; ok {
			fmt.Printf("%-15s %-12s %-12s %-10d %-15.2fx\n",
				string(result.EncodingType),
				formatDuration(result.EncodeTime),
				formatDuration(result.DecodeTime),
				result.EncodedSize,
				result.CompressionRatio)
		}
	}
	fmt.Println()
}

func printSizeComparison(testMsg *protocol.Message, results map[protocol.EncodingType]*protocol.BenchmarkResult) {
	fmt.Println("📊 消息大小详细对比:")
	fmt.Println("========================================")

	// 显示原始消息内容
	msgBytes, _ := json.MarshalIndent(testMsg, "", "  ")
	fmt.Printf("原始消息结构:\n%s\n\n", string(msgBytes))

	// 显示各协议编码后的内容示例
	factory := protocol.NewEncoderFactory()

	for encodingType, result := range results {
		encoder, _ := factory.GetEncoder(encodingType)
		encoded, _ := encoder.Encode(testMsg)

		fmt.Printf("%s 编码结果:\n", string(encodingType))
		fmt.Printf("  大小: %d 字节\n", len(encoded))
		fmt.Printf("  压缩比: %.2fx\n", result.CompressionRatio)

		if encodingType == protocol.EncodingJSON {
			// JSON可以直接显示
			fmt.Printf("  内容预览: %s\n", string(encoded)[:min(200, len(encoded))])
		} else {
			// 二进制格式显示十六进制
			fmt.Printf("  内容预览(hex): %x...\n", encoded[:min(50, len(encoded))])
		}
		fmt.Println()
	}
}

func printRecommendations() {
	fmt.Println("🎯 协议选择建议:")
	fmt.Println("========================================")

	recommendations := []struct {
		scenario string
		protocol string
		reason   string
	}{
		{"Web开发、调试阶段", "JSON", "可读性强，易于调试，生态成熟"},
		{"高并发IM系统", "MessagePack", "性能好，兼容JSON，体积小30%"},
		{"移动端APP", "Protocol Buffers", "最小体积，最快速度，省流量"},
		{"微服务间通信", "Protocol Buffers", "强类型，向前兼容，性能最优"},
		{"IoT设备", "CBOR", "标准化，支持流式，适合嵌入式"},
		{"大数据场景", "Apache Avro", "Schema进化，大数据生态友好"},
	}

	for _, rec := range recommendations {
		fmt.Printf("📱 %-20s → %-15s (%s)\n", rec.scenario, rec.protocol, rec.reason)
	}

	fmt.Println("\n💡 渐进式迁移策略:")
	fmt.Println("  1. 开发阶段: 使用 JSON (便于调试)")
	fmt.Println("  2. 测试阶段: 对比测试 MessagePack (性能优化)")
	fmt.Println("  3. 生产阶段: 根据负载选择最优协议")
	fmt.Println("  4. 客户端协商: 支持多协议，客户端自主选择")
}

func formatDuration(d time.Duration) string {
	if d < time.Microsecond {
		return fmt.Sprintf("%dns", d.Nanoseconds())
	} else if d < time.Millisecond {
		return fmt.Sprintf("%.1fμs", float64(d.Nanoseconds())/1000)
	} else {
		return fmt.Sprintf("%.1fms", float64(d.Nanoseconds())/1000000)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func init() {
	// 设置日志格式
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}
