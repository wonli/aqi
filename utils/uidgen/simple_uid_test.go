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
		if len(cast.ToString(id)) != 16 {
			invalidLength++
			t.Errorf("ID长度不正确: %d", id)
		}
	}
	log.Printf("生成完成，耗时: %v, 总数=%d, 唯一=%d, 重复=%d, 长度错误=%d, 速率=%.2f ID/秒\n", duration, total, len(seen), duplicates, invalidLength, float64(total)/duration.Seconds())
	if duplicates > 0 || invalidLength > 0 {
		t.Fatalf("并发生成校验失败: duplicates=%d invalidLength=%d", duplicates, invalidLength)
	}
}

func TestCrossSecondBoundary(t *testing.T) {
	now := time.Now()
	nextSecond := now.Truncate(time.Second).Add(time.Second)
	time.Sleep(time.Until(nextSecond.Add(-100 * time.Millisecond)))

	const goroutines = 500
	results := make(chan int64, goroutines)
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- GenId() }()
	}
	wg.Wait()
	close(results)
	assertUniqueInt64(t, results, "跨秒边界")
}

func TestExtremeStress(t *testing.T) {
	const goroutines = 5000
	const iterations = 200
	results := make(chan int64, goroutines*iterations)
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ { results <- GenId() }
		}()
	}
	wg.Wait()
	close(results)
	assertUniqueInt64(t, results, "极限压力")
}

func TestSameMillisecondConcurrency(t *testing.T) {
	const goroutines = 10000
	results := make(chan int64, goroutines)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); <-start; results <- GenId() }()
	}
	close(start)
	wg.Wait()
	close(results)
	assertUniqueInt64(t, results, "同一毫秒")
}

func TestUltraExtremeStress(t *testing.T) {
	const goroutines = 20000
	const iterations = 10
	results := make(chan int64, goroutines*iterations)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done(); <-start
			for j := 0; j < iterations; j++ { results <- GenId() }
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	assertUniqueInt64(t, results, "超极限压力")
}

func TestContinuousSecondBoundaryStress(t *testing.T) {
	const rounds = 5
	const goroutinesPerRound = 5000
	all := make(chan int64, rounds*goroutinesPerRound)
	for round := 0; round < rounds; round++ {
		now := time.Now()
		nextSecond := now.Truncate(time.Second).Add(time.Second)
		time.Sleep(time.Until(nextSecond.Add(-50 * time.Millisecond)))
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := 0; i < goroutinesPerRound; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); <-start; all <- GenId() }()
		}
		close(start)
		wg.Wait()
	}
	close(all)
	assertUniqueInt64(t, all, "连续秒边界")
}

func TestLongRunningStress(t *testing.T) {
	if testing.Short() { t.Skip("跳过长时间测试") }
	const duration = 5 * time.Second
	const goroutines = 1000
	results := make(chan int64, 1000000)
	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop: return
				case results <- GenId(): time.Sleep(time.Microsecond)
				}
			}
		}()
	}
	time.Sleep(duration)
	close(stop)
	wg.Wait()
	close(results)
	assertUniqueInt64(t, results, "长时间压力")
}

func TestExceedSingleSecondLimit(t *testing.T) {
	const targetIds = 150000
	const maxGoroutines = 50000
	const idsPerGoroutine = targetIds / maxGoroutines
	results := make(chan int64, targetIds)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < maxGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done(); <-start
			for j := 0; j < idsPerGoroutine; j++ { results <- GenId() }
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	assertUniqueInt64(t, results, "单秒极限")
}

func TestRealWorldMixedStress(t *testing.T) {
	const duration = 5 * time.Second
	const normalGoroutines = 100
	const burstGoroutines = 2000
	results := make(chan int64, 500000)
	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < normalGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-stop: return
				default: results <- GenId(); time.Sleep(time.Millisecond * time.Duration(10+id%20))
				}
			}
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(time.Second); defer ticker.Stop()
		for {
			select {
			case <-stop: return
			case <-ticker.C:
				var burst sync.WaitGroup
				for i := 0; i < burstGoroutines; i++ {
					burst.Add(1); go func() { defer burst.Done(); results <- GenId() }()
				}
				burst.Wait()
			}
		}
	}()
	time.Sleep(duration)
	close(stop)
	wg.Wait()
	close(results)
	assertUniqueInt64(t, results, "混合压力")
}

func TestExtremeMultiCoreStress(t *testing.T) {
	log.Println("开始最极端的多核心并发测试...")
	numCPU := runtime.NumCPU()
	runtime.GOMAXPROCS(numCPU)
	log.Printf("使用 %d 个CPU核心\n", numCPU)

	// LockOSThread 会让每个 goroutine 占用一个 OS thread；控制规模，避免测试耗尽系统线程。
	const goroutinesPerCore = 100
	const idsPerGoroutine = 100
	totalGoroutines := numCPU * goroutinesPerCore
	totalIds := totalGoroutines * idsPerGoroutine
	log.Printf("启动 %d 个goroutine (每核心%d个)，总共生成 %d 个ID\n", totalGoroutines, goroutinesPerCore, totalIds)

	results := make(chan int64, totalIds)
	var wg sync.WaitGroup
	startSignal := make(chan struct{})
	for core := 0; core < numCPU; core++ {
		for i := 0; i < goroutinesPerCore; i++ {
			wg.Add(1)
			go func(coreID, goroutineID int) {
				defer wg.Done()
				runtime.LockOSThread()
				defer runtime.UnlockOSThread()
				<-startSignal
				for j := 0; j < idsPerGoroutine; j++ { results <- GenId() }
			}(core, i)
		}
	}
	close(startSignal)
	wg.Wait()
	close(results)
	assertUniqueInt64(t, results, "多核心极限")
}

func TestMaliciousTimeConflict(t *testing.T) {
	const rounds = 100
	const goroutinesPerRound = 1000
	all := make(chan int64, rounds*goroutinesPerRound)
	for round := 0; round < rounds; round++ {
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := 0; i < goroutinesPerRound; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); <-start; all <- GenId() }()
		}
		time.Sleep(time.Microsecond * time.Duration(rand.Intn(1000)))
		close(start)
		wg.Wait()
	}
	close(all)
	assertUniqueInt64(t, all, "恶意时间冲突")
}

func TestPerformanceComparison(t *testing.T) {
	const iterations = 1000000
	const goroutines = 1000
	const idsPerGoroutine = iterations / goroutines
	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < goroutines; i++ {
		wg.Add(1); go func() { defer wg.Done(); for j := 0; j < idsPerGoroutine; j++ { _ = GenId() } }()
	}
	wg.Wait()
	log.Printf("GenId: %.2f ID/秒", float64(iterations)/time.Since(start).Seconds())
	start = time.Now()
	for i := 0; i < goroutines; i++ {
		wg.Add(1); go func() { defer wg.Done(); for j := 0; j < idsPerGoroutine; j++ { _ = GenSid() } }()
	}
	wg.Wait()
	log.Printf("GenSid: %.2f ID/秒", float64(iterations)/time.Since(start).Seconds())
}

func BenchmarkGenId(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) { for pb.Next() { _ = GenId() } })
}

func BenchmarkGenSid(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) { for pb.Next() { _ = GenSid() } })
}

func TestExtendedPerformanceComparison(t *testing.T) {
	if testing.Short() { t.Skip("跳过长时间性能测试") }
	const duration = 10 * time.Second
	const goroutines = 500
	for _, tc := range []struct{name string; gen func()}{
		{name: "GenId", gen: func(){ _ = GenId() }},
		{name: "GenSid", gen: func(){ _ = GenSid() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var wg sync.WaitGroup
			stop := make(chan struct{})
			for i := 0; i < goroutines; i++ {
				wg.Add(1); go func(){ defer wg.Done(); for { select { case <-stop: return; default: tc.gen() } } }()
			}
			time.Sleep(duration); close(stop); wg.Wait()
		})
	}
}

func assertUniqueInt64(t *testing.T, results <-chan int64, name string) {
	t.Helper()
	seen := make(map[int64]struct{})
	for id := range results {
		if _, ok := seen[id]; ok { t.Fatalf("%s测试发现重复ID: %d", name, id) }
		seen[id] = struct{}{}
	}
	log.Printf("%s测试结果: 总数=%d, 唯一=%d", name, len(seen), len(seen))
}
