package game

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net"
	"net/http"
	"strconv"
	"time"
)

// StartMockServer เริ่มต้นรันเซิร์ฟเวอร์จำลองเพื่อเสิร์ฟข้อมูลแผนที่และรับรายงานแจ้งเตือนความผิดพลาด
func StartMockServer() (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}

	port := listener.Addr().(*net.TCPAddr).Port
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	mux := http.NewServeMux()
	
	// API ดึงข้อมูลด่าน
	mux.HandleFunc("/level", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		idStr := r.URL.Query().Get("id")
		id := 1
		if val, err := strconv.Atoi(idStr); err == nil {
			id = val
		}

		tiles, items := getBaseLevelLayout(id)

		candidates := scanPartitionWalls(tiles)
		count := getFakeWallCount(id)

		if count > len(candidates) {
			count = len(candidates)
		}

		if count > 0 {
			rand.Shuffle(len(candidates), func(i, j int) {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			})

			for i := 0; i < count; i++ {
				pt := candidates[i]
				tiles[pt.y][pt.x] = 7 // TileFakeWall
			}
		}

		levelData := map[string]interface{}{
			"id":     id,
			"width":  12,
			"height": 12,
			"tiles":  tiles,
			"items":  items,
		}

		json.NewEncoder(w).Encode(levelData)
	})

	// API รับรายงานแจ้งเตือนข้อผิดพลาดจากชุดทดสอบ (Backend Failure Alert Router)
	mux.HandleFunc("/report", func(w http.ResponseWriter, r *http.Request) {
		errType := r.URL.Query().Get("error")
		var payload CategorizedReportPayload
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&payload)
		}
		if errType == "" && payload.Category != "" {
			errType = payload.Category
		}
		fmt.Printf("\n🚨 [BACKEND ALERT] CRITICAL ERROR ENCOUNTERED: Category='%s', FPS=%.1f, TPS=%.1f, LevelID=%d, Details='%s'\n\n",
			errType, payload.FPS, payload.TPS, payload.LevelID, payload.Details)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","message":"Backend received crash report successfully"}`))
	})

	go func() {
		http.Serve(listener, mux)
	}()

	return baseURL, nil
}

type CategorizedReportPayload struct {
	Category  string  `json:"category"`
	FPS       float64 `json:"fps,omitempty"`
	TPS       float64 `json:"tps,omitempty"`
	LevelID   int     `json:"level_id,omitempty"`
	Timestamp int64   `json:"timestamp,omitempty"`
	Details   string  `json:"details,omitempty"`
}

func SendCategorizedReport(baseURL string, category string, fps, tps float64, levelID int, details string) error {
	if baseURL == "" {
		return fmt.Errorf("base URL is empty")
	}

	payload := CategorizedReportPayload{
		Category:  category,
		FPS:       fps,
		TPS:       tps,
		LevelID:   levelID,
		Timestamp: time.Now().Unix(),
		Details:   details,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal report payload: %w", err)
	}

	url := fmt.Sprintf("%s/report?error=%s", baseURL, category)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(bodyBytes))
	if err != nil {
		return fmt.Errorf("http post to /report failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned non-200 status: %d", resp.StatusCode)
	}

	return nil
}

func FetchLevelFromNetwork(url string) (*Level, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server error status: %d", resp.StatusCode)
	}

	var level Level
	if err := json.NewDecoder(resp.Body).Decode(&level); err != nil {
		return nil, err
	}
	return &level, nil
}

type point struct {
	x, y int
}

func scanPartitionWalls(tiles [][]int) []point {
	var candidates []point
	height := len(tiles)
	if height == 0 {
		return candidates
	}
	width := len(tiles[0])

	for y := 1; y < height-1; y++ {
		for x := 1; x < width-1; x++ {
			if tiles[y][x] == 1 {
				leftRight := tiles[y][x-1] == 0 && tiles[y][x+1] == 0
				upDown := tiles[y-1][x] == 0 && tiles[y+1][x] == 0

				if leftRight || upDown {
					candidates = append(candidates, point{x: x, y: y})
				}
			}
		}
	}
	return candidates
}

func getFakeWallCount(id int) int {
	if id == 1 {
		return 2
	}
	res := 1
	for i := 0; i < id; i++ {
		res *= id
	}
	return res
}

func getBaseLevelLayout(id int) ([][]int, []map[string]interface{}) {
	var tiles [][]int
	var items []map[string]interface{}

	switch id {
	case 2:
		tiles = [][]int{
			{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
			{1, 9, 0, 1, 0, 0, 0, 0, 0, 0, 0, 1},
			{1, 0, 10, 1, 0, 1, 1, 1, 1, 1, 0, 1},
			{1, 1, 0, 1, 0, 1, 6, 1, 1, 1, 0, 1},
			{1, 0, 0, 0, 0, 1, 1, 1, 1, 1, 0, 1},
			{1, 0, 1, 1, 1, 1, 0, 0, 0, 1, 0, 1},
			{1, 0, 1, 0, 0, 0, 0, 1, 0, 1, 0, 1},
			{1, 0, 1, 0, 1, 1, 1, 1, 0, 1, 0, 1},
			{1, 0, 1, 5, 1, 1, 1, 1, 0, 1, 0, 1},
			{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
			{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 8, 1},
			{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
		}
		items = []map[string]interface{}{
			{"x": 10, "y": 1, "type": 2},
		}

	case 3:
		tiles = [][]int{
			{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
			{1, 9, 10, 0, 0, 0, 0, 0, 0, 0, 0, 1},
			{1, 0, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1},
			{1, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 1},
			{1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 0, 1},
			{1, 0, 0, 0, 0, 0, 0, 1, 0, 1, 0, 1},
			{1, 1, 1, 1, 1, 1, 0, 1, 0, 1, 0, 1},
			{1, 0, 0, 0, 5, 0, 0, 1, 0, 1, 0, 1},
			{1, 0, 0, 0, 4, 0, 0, 0, 0, 1, 0, 1},
			{1, 0, 0, 0, 1, 1, 1, 1, 1, 1, 0, 1},
			{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 8, 1},
			{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
		}
		items = []map[string]interface{}{
			{"x": 10, "y": 1, "type": 1},
			{"x": 1, "y": 5, "type": 2},
		}

	case 4:
		tiles = [][]int{
			{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
			{1, 9, 10, 0, 0, 1, 0, 0, 0, 0, 0, 1},
			{1, 1, 1, 1, 0, 1, 0, 1, 1, 1, 0, 1},
			{1, 0, 0, 0, 0, 0, 0, 1, 0, 1, 0, 1},
			{1, 0, 1, 1, 1, 1, 1, 1, 0, 1, 0, 1},
			{1, 0, 1, 0, 0, 0, 0, 5, 0, 1, 0, 1},
			{1, 0, 1, 0, 1, 1, 1, 1, 1, 1, 0, 1},
			{1, 0, 1, 0, 1, 6, 0, 0, 0, 0, 0, 1},
			{1, 0, 0, 0, 1, 1, 1, 1, 1, 1, 0, 1},
			{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
			{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 3, 1},
			{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
		}
		items = []map[string]interface{}{
			{"x": 1, "y": 3, "type": 2},
			{"x": 10, "y": 10, "type": 3},
		}

	default:
		tiles = [][]int{
			{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
			{1, 0, 0, 10, 1, 6, 1, 0, 0, 0, 0, 1}, // (3,1) Item Box, (5,1) Boss 'ต้น'
			{1, 0, 1, 0, 1, 0, 1, 0, 1, 1, 0, 1},
			{1, 0, 1, 0, 0, 0, 0, 0, 0, 1, 0, 1},
			{1, 0, 1, 1, 1, 5, 1, 1, 0, 1, 0, 1}, // (5,4) Blue Door
			{1, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 1},
			{1, 1, 1, 1, 0, 1, 1, 1, 0, 1, 0, 1},
			{1, 0, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1},
			{1, 0, 0, 1, 4, 1, 0, 1, 0, 1, 0, 1}, // (4,8) Red Door
			{1, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0, 1},
			{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 8, 1}, // (10,10) Stairs Up to 2F
			{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
		}
		items = []map[string]interface{}{
			{"x": 10, "y": 1, "type": 1}, // Red Key at (10,1)
		}
	}

	tilesCopy := make([][]int, len(tiles))
	for i := range tiles {
		tilesCopy[i] = make([]int, len(tiles[i]))
		copy(tilesCopy[i], tiles[i])
	}

	return tilesCopy, items
}
