package uidgen

import (
	"log"
	"math/rand"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cast"
)

func TestGenerateID(t *testing.T) {
	ddd := map[int64]bool{}

	for i := 0; i < 800000; i++ {
		tid := GenId()
		lll := len(cast.ToString(tid))
		if ddd[tid] || lll != 16 {
			t.Errorf("测试失败")
			log.Println(tid, " -- ", len(cast.ToString(tid)))
		} else {
			ddd[tid] = true
		}
	}

	log.Println(len(ddd))
}

func TestConcurrentGenId(t *testing.T) {
	const goroutines = 1000
	const iterations = 100

	results := make(chan int64, goroutines*iterations)
	var wg sync.WaitGroup

	log.Printf("开始高并发测试: %d个goroutine，每个生成%d个ID\n", goroutines, iterations)
	start := time.Now()

	// 真正的高并发测试
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				results <- GenId()
				// 不要sleep，保持真正的并发压力
			}
		}()
	}

	wg.Wait()
	close(results)

	duration := time.Since(start)
	log.Printf("生成完成，耗时: %v\n", duration)

	// 检查重复和长度
	seen := make(map[int64]bool)
	duplicates := 0
	invalidLength := 0
	total := 0

	for id := range results {
		total++
		if seen[id] {
			duplicates++
			t.Errorf("发现重复ID: %d", id)
		}
		seen[id] = true

		idStr := cast.ToString(id)
		if len(idStr) != 16 {
			invalidLength++
			t.Errorf("ID长度不正确: %d (长度: %d)", id, len(idStr))
		}
	}

	log.Printf("测试结果统计:\n")
	log.Printf("- 总生成数量: %d\n", total)
	log.Printf("- 唯一ID数量: %d\n", len(seen))
	log.Printf("- 重复ID数量: %d\n", duplicates)
	log.Printf("- 长度错误数量: %d\n", invalidLength)
	log.Printf("- 生成速率: %.2f ID/秒\n", float64(total)/duration.Seconds())

	if duplicates > 0 {
		t.Fatalf("发现 %d 个重复ID，测试失败！", duplicates)
	}

	if invalidLength > 0 {
		t.Fatalf("发现 %d 个长度错误的ID，测试失败！", invalidLength)
	}
}

// 跨秒边界测试
func TestCrossSecondBoundary(t *testing.T) {
	log.Println("开始跨秒边界测试...")

	// 等待接近下一秒
	now := time.Now()
	nextSecond := now.Truncate(time.Second).Add(time.Second)
	time.Sleep(time.Until(nextSecond.Add(-100 * time.Millisecond)))

	const goroutines = 500
	results := make(chan int64, goroutines)
	var wg sync.WaitGroup

	// 在秒边界附近启动大量goroutine
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- GenId()
		}()
	}

	wg.Wait()
	close(results)

	// 检查重复
	seen := make(map[int64]bool)
	duplicates := 0
	total := 0

	for id := range results {
		total++
		if seen[id] {
			duplicates++
			t.Errorf("跨秒边界测试发现重复ID: %d", id)
		}
		seen[id] = true
	}

	log.Printf("跨秒边界测试结果: 总数=%d, 唯一=%d, 重复=%d\n", total, len(seen), duplicates)

	if duplicates > 0 {
		t.Fatalf("跨秒边界测试发现 %d 个重复ID！", duplicates)
	}
}

// 极限压力测试
func TestExtremeStress(t *testing.T) {
	const goroutines = 5000
	const iterations = 200

	log.Printf("开始极限压力测试: %d个goroutine，每个生成%d个ID\n", goroutines, iterations)
	start := time.Now()

	results := make(chan int64, goroutines*iterations)
	var wg sync.WaitGroup

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				results <- GenId()
			}
		}()
	}

	wg.Wait()
	close(results)

	duration := time.Since(start)
	log.Printf("极限压力测试完成，耗时: %v\n", duration)

	// 检查重复
	seen := make(map[int64]bool)
	duplicates := 0
	total := 0

	for id := range results {
		total++
		if seen[id] {
			duplicates++
			t.Errorf("极限压力测试发现重复ID: %d", id)
		}
		seen[id] = true
	}

	log.Printf("极限压力测试结果: 总数=%d, 唯一=%d, 重复=%d, 速率=%.2f ID/秒\n",
		total, len(seen), duplicates, float64(total)/duration.Seconds())

	if duplicates > 0 {
		t.Fatalf("极限压力测试发现 %d 个重复ID！", duplicates)
	}
}

// 同一毫秒内的超高并发测试
func TestSameMillisecondConcurrency(t *testing.T) {
	log.Println("开始同一毫秒内超高并发测试...")

	const goroutines = 10000
	results := make(chan int64, goroutines)
	var wg sync.WaitGroup
	var startSignal sync.WaitGroup
	startSignal.Add(1)

	// 预先启动所有goroutine，等待信号同时开始
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			startSignal.Wait() // 等待开始信号
			results <- GenId()
		}()
	}

	// 短暂延迟后同时释放所有goroutine
	time.Sleep(10 * time.Millisecond)
	startSignal.Done()

	wg.Wait()
	close(results)

	// 检查重复
	seen := make(map[int64]bool)
	duplicates := 0
	total := 0

	for id := range results {
		total++
		if seen[id] {
			duplicates++
			t.Errorf("同一毫秒测试发现重复ID: %d", id)
		}
		seen[id] = true
	}

	log.Printf("同一毫秒测试结果: 总数=%d, 唯一=%d, 重复=%d\n", total, len(seen), duplicates)

	if duplicates > 0 {
		t.Fatalf("同一毫秒测试发现 %d 个重复ID！", duplicates)
	}
}

// 超极限压力测试 - 挑战单秒10万次限制
func TestUltraExtremeStress(t *testing.T) {
	const goroutines = 20000
	const iterations = 10

	log.Printf("开始超极限压力测试: %d个goroutine，每个生成%d个ID (总计%d个)\n",
		goroutines, iterations, goroutines*iterations)
	start := time.Now()

	results := make(chan int64, goroutines*iterations)
	var wg sync.WaitGroup
	var startSignal sync.WaitGroup
	startSignal.Add(1)

	// 预先启动所有goroutine，等待信号同时开始
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			startSignal.Wait() // 等待开始信号
			for j := 0; j < iterations; j++ {
				results <- GenId()
			}
		}()
	}

	// 同时释放所有goroutine，制造极端并发
	startSignal.Done()

	wg.Wait()
	close(results)

	duration := time.Since(start)
	log.Printf("超极限压力测试完成，耗时: %v\n", duration)

	// 检查重复
	seen := make(map[int64]bool)
	duplicates := 0
	total := 0
	duplicateIds := make([]int64, 0)

	for id := range results {
		total++
		if seen[id] {
			duplicates++
			duplicateIds = append(duplicateIds, id)
			t.Errorf("超极限压力测试发现重复ID: %d", id)
		}
		seen[id] = true
	}

	log.Printf("超极限压力测试结果: 总数=%d, 唯一=%d, 重复=%d, 速率=%.2f ID/秒\n",
		total, len(seen), duplicates, float64(total)/duration.Seconds())

	if len(duplicateIds) > 0 {
		log.Printf("重复的ID列表: %v\n", duplicateIds[:min(len(duplicateIds), 10)])
	}

	if duplicates > 0 {
		t.Fatalf("超极限压力测试发现 %d 个重复ID！", duplicates)
	}
}

// 连续秒边界冲击测试
func TestContinuousSecondBoundaryStress(t *testing.T) {
	log.Println("开始连续秒边界冲击测试...")

	const rounds = 5
	const goroutinesPerRound = 5000
	allResults := make([]int64, 0, rounds*goroutinesPerRound)
	var mu sync.Mutex

	for round := 0; round < rounds; round++ {
		log.Printf("第 %d 轮秒边界测试\n", round+1)

		// 等待接近下一秒
		now := time.Now()
		nextSecond := now.Truncate(time.Second).Add(time.Second)
		time.Sleep(time.Until(nextSecond.Add(-50 * time.Millisecond)))

		results := make(chan int64, goroutinesPerRound)
		var wg sync.WaitGroup
		var startSignal sync.WaitGroup
		startSignal.Add(1)

		// 在秒边界附近启动大量goroutine
		for i := 0; i < goroutinesPerRound; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				startSignal.Wait()
				results <- GenId()
			}()
		}

		startSignal.Done()
		wg.Wait()
		close(results)

		// 收集结果
		mu.Lock()
		for id := range results {
			allResults = append(allResults, id)
		}
		mu.Unlock()
	}

	// 检查所有轮次的重复
	seen := make(map[int64]bool)
	duplicates := 0
	duplicateIds := make([]int64, 0)

	for _, id := range allResults {
		if seen[id] {
			duplicates++
			duplicateIds = append(duplicateIds, id)
			t.Errorf("连续秒边界测试发现重复ID: %d", id)
		}
		seen[id] = true
	}

	log.Printf("连续秒边界测试结果: 总数=%d, 唯一=%d, 重复=%d\n",
		len(allResults), len(seen), duplicates)

	if len(duplicateIds) > 0 {
		log.Printf("重复的ID列表: %v\n", duplicateIds[:min(len(duplicateIds), 10)])
	}

	if duplicates > 0 {
		t.Fatalf("连续秒边界测试发现 %d 个重复ID！", duplicates)
	}
}

// 长时间持续压力测试
func TestLongRunningStress(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过长时间测试")
	}

	log.Println("开始长时间持续压力测试 (5秒)...")

	const duration = 5 * time.Second
	const goroutines = 1000

	results := make(chan int64, 1000000) // 预分配大缓冲区
	var wg sync.WaitGroup
	stop := make(chan struct{})

	start := time.Now()

	// 启动持续生成ID的goroutine
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				case results <- GenId():
					time.Sleep(time.Microsecond) // 微小延迟避免CPU占用过高
				}
			}
		}()
	}

	// 运行指定时间后停止
	time.Sleep(duration)
	close(stop)
	wg.Wait()
	close(results)

	actualDuration := time.Since(start)
	log.Printf("长时间测试完成，实际耗时: %v\n", actualDuration)

	// 检查重复
	seen := make(map[int64]bool)
	duplicates := 0
	total := 0
	duplicateIds := make([]int64, 0)

	for id := range results {
		total++
		if seen[id] {
			duplicates++
			duplicateIds = append(duplicateIds, id)
			t.Errorf("长时间测试发现重复ID: %d", id)
		}
		seen[id] = true
	}

	log.Printf("长时间测试结果: 总数=%d, 唯一=%d, 重复=%d, 平均速率=%.2f ID/秒\n",
		total, len(seen), duplicates, float64(total)/actualDuration.Seconds())

	if len(duplicateIds) > 0 {
		log.Printf("重复的ID列表: %v\n", duplicateIds[:min(len(duplicateIds), 10)])
	}

	if duplicates > 0 {
		t.Fatalf("长时间测试发现 %d 个重复ID！", duplicates)
	}
}

// 单秒内超过10万次调用的极限测试
func TestExceedSingleSecondLimit(t *testing.T) {
	log.Println("开始单秒内超过10万次调用的极限测试...")

	// 目标：在1秒内生成超过10万个ID
	const targetIds = 150000
	const maxGoroutines = 50000
	const idsPerGoroutine = targetIds / maxGoroutines

	log.Printf("目标生成 %d 个ID，使用 %d 个goroutine，每个生成 %d 个\n",
		targetIds, maxGoroutines, idsPerGoroutine)

	results := make(chan int64, targetIds)
	var wg sync.WaitGroup
	var startSignal sync.WaitGroup
	startSignal.Add(1)

	start := time.Now()

	// 启动大量goroutine
	for i := 0; i < maxGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			startSignal.Wait()
			for j := 0; j < idsPerGoroutine; j++ {
				results <- GenId()
			}
		}()
	}

	// 同时释放所有goroutine
	startSignal.Done()
	wg.Wait()
	close(results)

	duration := time.Since(start)
	log.Printf("极限测试完成，耗时: %v\n", duration)

	// 检查重复
	seen := make(map[int64]bool)
	duplicates := 0
	total := 0
	duplicateIds := make([]int64, 0)

	for id := range results {
		total++
		if seen[id] {
			duplicates++
			duplicateIds = append(duplicateIds, id)
			t.Errorf("单秒极限测试发现重复ID: %d", id)
		}
		seen[id] = true
	}

	log.Printf("单秒极限测试结果: 总数=%d, 唯一=%d, 重复=%d, 速率=%.2f ID/秒\n",
		total, len(seen), duplicates, float64(total)/duration.Seconds())

	if len(duplicateIds) > 0 {
		log.Printf("重复的ID列表 (前10个): %v\n", duplicateIds[:min(len(duplicateIds), 10)])
		// 分析重复ID的模式
		log.Printf("分析重复ID模式...\n")
		for i, dupId := range duplicateIds[:min(len(duplicateIds), 5)] {
			idStr := cast.ToString(dupId)
			log.Printf("重复ID %d: %s (长度: %d)\n", i+1, idStr, len(idStr))
		}
	}

	if duplicates > 0 {
		t.Fatalf("单秒极限测试发现 %d 个重复ID！这证明了在超高并发下存在唯一性问题", duplicates)
	} else {
		log.Printf("✅ 即使在 %.2f ID/秒 的极高速率下，仍然保持了唯一性\n", float64(total)/duration.Seconds())
	}
}

// 模拟真实业务场景的混合压力测试
func TestRealWorldMixedStress(t *testing.T) {
	log.Println("开始模拟真实业务场景的混合压力测试...")

	const duration = 5 * time.Second
	const normalGoroutines = 100          // 正常业务goroutine
	const burstGoroutines = 2000          // 突发流量goroutine
	const burstInterval = 1 * time.Second // 突发间隔

	results := make(chan int64, 500000)
	var wg sync.WaitGroup
	stop := make(chan struct{})

	start := time.Now()

	// 启动正常业务流量
	for i := 0; i < normalGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					results <- GenId()
					time.Sleep(time.Millisecond * time.Duration(10+id%20)) // 模拟不同的业务处理时间
				}
			}
		}(i)
	}

	// 定期产生突发流量
	go func() {
		ticker := time.NewTicker(burstInterval)
		defer ticker.Stop()

		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				log.Printf("触发突发流量: %d 个并发请求\n", burstGoroutines)
				// 突发大量并发请求
				var burstWg sync.WaitGroup
				for i := 0; i < burstGoroutines; i++ {
					burstWg.Add(1)
					go func() {
						defer burstWg.Done()
						results <- GenId()
					}()
				}
				burstWg.Wait()
			}
		}
	}()

	// 运行指定时间
	time.Sleep(duration)
	close(stop)
	wg.Wait()
	close(results)

	actualDuration := time.Since(start)
	log.Printf("混合压力测试完成，实际耗时: %v\n", actualDuration)

	// 检查重复
	seen := make(map[int64]bool)
	duplicates := 0
	total := 0
	duplicateIds := make([]int64, 0)

	for id := range results {
		total++
		if seen[id] {
			duplicates++
			duplicateIds = append(duplicateIds, id)
			t.Errorf("混合压力测试发现重复ID: %d", id)
		}
		seen[id] = true
	}

	log.Printf("混合压力测试结果: 总数=%d, 唯一=%d, 重复=%d, 平均速率=%.2f ID/秒\n",
		total, len(seen), duplicates, float64(total)/actualDuration.Seconds())

	if len(duplicateIds) > 0 {
		log.Printf("重复的ID列表: %v\n", duplicateIds[:min(len(duplicateIds), 10)])
	}

	if duplicates > 0 {
		t.Fatalf("混合压力测试发现 %d 个重复ID！", duplicates)
	} else {
		log.Printf("✅ 在真实业务场景模拟下保持了唯一性\n")
	}
}

// 最极端的多核心并发测试
func TestExtremeMultiCoreStress(t *testing.T) {
	log.Println("开始最极端的多核心并发测试...")

	// 获取CPU核心数
	numCPU := runtime.NumCPU()
	runtime.GOMAXPROCS(numCPU)
	log.Printf("使用 %d 个CPU核心\n", numCPU)

	// 每个核心启动合理数量的goroutine
	const goroutinesPerCore = 1000
	const idsPerGoroutine = 100
	totalGoroutines := numCPU * goroutinesPerCore
	totalIds := totalGoroutines * idsPerGoroutine

	log.Printf("启动 %d 个goroutine (每核心%d个)，总共生成 %d 个ID\n",
		totalGoroutines, goroutinesPerCore, totalIds)

	results := make(chan int64, totalIds)
	var wg sync.WaitGroup
	var startSignal sync.WaitGroup
	startSignal.Add(1)

	start := time.Now()

	// 为每个CPU核心启动goroutine
	for core := 0; core < numCPU; core++ {
		for i := 0; i < goroutinesPerCore; i++ {
			wg.Add(1)
			go func(coreId, goroutineId int) {
				defer wg.Done()
				// 绑定到特定CPU核心（尽力而为）
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()

				startSignal.Wait()
				for j := 0; j < idsPerGoroutine; j++ {
					results <- GenId()
					// 故意不加任何延迟，最大化并发冲突
				}
			}(core, i)
		}
	}

	// 同时释放所有goroutine
	startSignal.Done()
	wg.Wait()
	close(results)

	duration := time.Since(start)
	log.Printf("多核心极限测试完成，耗时: %v\n", duration)

	// 检查重复
	seen := make(map[int64]bool)
	duplicates := 0
	total := 0
	duplicateIds := make([]int64, 0)

	for id := range results {
		total++
		if seen[id] {
			duplicates++
			duplicateIds = append(duplicateIds, id)
			t.Errorf("多核心极限测试发现重复ID: %d", id)
		}
		seen[id] = true
	}

	log.Printf("多核心极限测试结果: 总数=%d, 唯一=%d, 重复=%d, 速率=%.2f ID/秒\n",
		total, len(seen), duplicates, float64(total)/duration.Seconds())

	if len(duplicateIds) > 0 {
		log.Printf("重复的ID列表 (前10个): %v\n", duplicateIds[:min(len(duplicateIds), 10)])
	}

	if duplicates > 0 {
		t.Fatalf("多核心极限测试发现 %d 个重复ID！", duplicates)
	} else {
		log.Printf("✅ 在 %d 核心、%.2f ID/秒 的极限条件下仍然保持唯一性\n",
			numCPU, float64(total)/duration.Seconds())
	}
}

// 故意制造时间冲突的恶意测试
func TestMaliciousTimeConflict(t *testing.T) {
	log.Println("开始故意制造时间冲突的恶意测试...")

	// 这个测试试图在完全相同的时间点生成ID
	const rounds = 100
	const goroutinesPerRound = 1000

	allResults := make([]int64, 0, rounds*goroutinesPerRound)
	var globalMutex sync.Mutex

	for round := 0; round < rounds; round++ {
		log.Printf("第 %d 轮时间冲突测试\n", round+1)

		results := make(chan int64, goroutinesPerRound)
		var wg sync.WaitGroup
		var startSignal sync.WaitGroup
		startSignal.Add(1)

		// 启动goroutine但不让它们开始
		for i := 0; i < goroutinesPerRound; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				startSignal.Wait()
				// 在完全相同的时刻调用GenId
				results <- GenId()
			}()
		}

		// 等待一个随机的短时间，然后同时释放所有goroutine
		time.Sleep(time.Microsecond * time.Duration(rand.Intn(1000)))
		startSignal.Done()
		wg.Wait()
		close(results)

		// 收集这一轮的结果
		roundResults := make([]int64, 0, goroutinesPerRound)
		for id := range results {
			roundResults = append(roundResults, id)
		}

		globalMutex.Lock()
		allResults = append(allResults, roundResults...)
		globalMutex.Unlock()

		// 检查这一轮内部是否有重复
		roundSeen := make(map[int64]bool)
		roundDuplicates := 0
		for _, id := range roundResults {
			if roundSeen[id] {
				roundDuplicates++
				t.Errorf("第%d轮发现重复ID: %d", round+1, id)
			}
			roundSeen[id] = true
		}

		if roundDuplicates > 0 {
			log.Printf("⚠️  第%d轮发现 %d 个重复ID\n", round+1, roundDuplicates)
		}
	}

	// 检查全局重复
	log.Println("检查全局重复...")
	globalSeen := make(map[int64]bool)
	globalDuplicates := 0
	duplicateIds := make([]int64, 0)

	for _, id := range allResults {
		if globalSeen[id] {
			globalDuplicates++
			duplicateIds = append(duplicateIds, id)
			t.Errorf("恶意时间冲突测试发现重复ID: %d", id)
		}
		globalSeen[id] = true
	}

	log.Printf("恶意时间冲突测试结果: 总数=%d, 唯一=%d, 重复=%d\n",
		len(allResults), len(globalSeen), globalDuplicates)

	if len(duplicateIds) > 0 {
		log.Printf("重复的ID列表: %v\n", duplicateIds[:min(len(duplicateIds), 20)])
	}

	if globalDuplicates > 0 {
		t.Fatalf("恶意时间冲突测试发现 %d 个重复ID！这证明了在极端时间冲突下存在问题", globalDuplicates)
	} else {
		log.Printf("✅ 即使在故意制造的时间冲突下仍然保持唯一性\n")
	}
}

// 性能对比测试：int64 vs string 版本
func TestPerformanceComparison(t *testing.T) {
	log.Println("开始性能对比测试: GenId (int64) vs GenSid (string)")

	const iterations = 1000000
	const goroutines = 1000
	const idsPerGoroutine = iterations / goroutines

	// 测试 GenId (int64版本)
	log.Printf("测试 GenId (int64版本): %d 个goroutine，每个生成 %d 个ID\n", goroutines, idsPerGoroutine)
	start1 := time.Now()

	var wg1 sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg1.Add(1)
		go func() {
			defer wg1.Done()
			for j := 0; j < idsPerGoroutine; j++ {
				_ = GenId()
			}
		}()
	}
	wg1.Wait()

	duration1 := time.Since(start1)
	rate1 := float64(iterations) / duration1.Seconds()
	log.Printf("GenId (int64) 结果: 耗时=%v, 速率=%.2f ID/秒\n", duration1, rate1)

	// 测试 GenSid (string版本)
	log.Printf("测试 GenSid (string版本): %d 个goroutine，每个生成 %d 个ID\n", goroutines, idsPerGoroutine)
	start2 := time.Now()

	var wg2 sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			for j := 0; j < idsPerGoroutine; j++ {
				_ = GenSid()
			}
		}()
	}
	wg2.Wait()

	duration2 := time.Since(start2)
	rate2 := float64(iterations) / duration2.Seconds()
	log.Printf("GenSid (string) 结果: 耗时=%v, 速率=%.2f ID/秒\n", duration2, rate2)

	// 性能对比分析
	speedupPercent := ((rate2 - rate1) / rate1) * 100
	log.Printf("\n📊 性能对比分析:\n")
	log.Printf("GenId (int64):     %.2f ID/秒\n", rate1)
	log.Printf("GenSid (str): %.2f ID/秒\n", rate2)

	if speedupPercent > 0 {
		log.Printf("✅ GenSid 比 GenId 快 %.2f%%\n", speedupPercent)
	} else {
		log.Printf("⚠️  GenSid 比 GenId 慢 %.2f%%\n", -speedupPercent)
	}

	// 验证两个版本生成的ID格式一致性
	log.Println("\n🔍 验证ID格式一致性:")
	for i := 0; i < 5; i++ {
		intId := GenId()
		strId := GenSid()
		log.Printf("GenId: %d (长度: %d), GenSid: %s (长度: %d)\n",
			intId, len(cast.ToString(intId)), strId, len(strId))
	}
}

// Benchmark测试
func BenchmarkGenId(b *testing.B) {
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = GenId()
		}
	})
}

func BenchmarkGenSid(b *testing.B) {
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = GenSid()
		}
	})
}

// 长时间性能对比测试 - 延长测试时间获得更准确的性能数据
func TestExtendedPerformanceComparison(t *testing.T) {
	log.Println("开始长时间性能对比测试: GenId (int64) vs GenSid (string)")

	const duration = 10 * time.Second // 延长到10秒
	const goroutines = 500

	// 测试GenId (int64版本)
	log.Printf("测试 GenId (int64版本): %d 个goroutine，持续 %v", goroutines, duration)
	start1 := time.Now()

	results1 := make(chan int64, 100000)
	var wg1 sync.WaitGroup
	stop1 := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		wg1.Add(1)
		go func() {
			defer wg1.Done()
			for {
				select {
				case <-stop1:
					return
				case results1 <- GenId():
					// 无延迟，最大化性能测试
				}
			}
		}()
	}

	time.Sleep(duration)
	close(stop1)
	wg1.Wait()
	close(results1)

	duration1 := time.Since(start1)
	count1 := len(results1)
	rate1 := float64(count1) / duration1.Seconds()

	log.Printf("GenId (int64) 结果: 耗时=%v, 总数=%d, 速率=%.2f ID/秒", duration1, count1, rate1)

	// 测试GenSid (string版本)
	log.Printf("测试 GenSid (string版本): %d 个goroutine，持续 %v", goroutines, duration)
	start2 := time.Now()

	results2 := make(chan string, 100000)
	var wg2 sync.WaitGroup
	stop2 := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			for {
				select {
				case <-stop2:
					return
				case results2 <- GenSid():
					// 无延迟，最大化性能测试
				}
			}
		}()
	}

	time.Sleep(duration)
	close(stop2)
	wg2.Wait()
	close(results2)

	duration2 := time.Since(start2)
	count2 := len(results2)
	rate2 := float64(count2) / duration2.Seconds()

	log.Printf("GenSid (string) 结果: 耗时=%v, 总数=%d, 速率=%.2f ID/秒", duration2, count2, rate2)

	// 性能对比分析
	log.Println("\n📊 长时间性能对比分析:")
	log.Printf("GenId (int64):     %.2f ID/秒", rate1)
	log.Printf("GenSid (str): %.2f ID/秒", rate2)

	if rate2 > rate1 {
		percentage := ((rate2 - rate1) / rate1) * 100
		log.Printf("✅ GenSid 比 GenId 快 %.2f%%", percentage)
	} else {
		percentage := ((rate1 - rate2) / rate2) * 100
		log.Printf("⚠️  GenId 比 GenSid 快 %.2f%%", percentage)
	}

	// 验证ID格式一致性
	log.Println("\n🔍 验证ID格式一致性:")
	for i := 0; i < 5; i++ {
		id1 := GenId()
		id2 := GenSid()
		log.Printf("GenId: %d (长度: %d), GenSid: %s (长度: %d)",
			id1, len(cast.ToString(id1)), id2, len(id2))
	}
}

// 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
