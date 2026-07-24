package game

import (
	"fmt"
	"image/color"
	"math"
	"os"
	"sort"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type Game struct {
	level             *Level
	player            *Player
	loading           bool
	err               error
	serverURL         string
	currentLevelID    int
	victory           bool
	tick              int
	characterSelected bool
	charSelectionRow  int


	// ระบบกล้องเลื่อนตาม 3D Perspective Software Engine
	camX float64
	camY float64
	camZ float64

	// ระบบช่องสล็อตกระเป๋าเดินทางสไตล์ RE0
	selectedSlot int

	// ระบบ Item Box Modal UI และคลังเก็บของรายด่าน
	boxUIOpen      bool
	boxActiveCol   int // 0: Player Inventory, 1: Item Box
	boxPlayerSlot  int // selected slot 0..4
	boxItemSlot    int // selected slot 0..(capacity-1)
	boxInventories map[int][]ItemType

	// ระบบรายงานประสิทธิภาพ FPS/TPS Drops
	lastFPSReportTime time.Time

	// ระบบ Pause และ Settings
	isPaused         bool
	simplePause      bool
	pauseActiveRow   int // 0: Resume, 1: Settings, 2: Reset Floor
	settingsOpen     bool
	settingActiveRow int // 0: Sound Effects, 1: Retro Filter, 2: Camera Speed, 3: Back
	soundMuted       bool
	retroFilter      bool
	cameraSpeed      float64
	lastZone         int
}

func NewGame() *Game {
	g := &Game{
		level:             NewDefaultLevel(),
		player:            NewPlayer(1, 1),
		loading:           false,
		currentLevelID:    1,
		selectedSlot:      0,
		boxInventories:    make(map[int][]ItemType),
		cameraSpeed:       0.08,
		lastZone:          0,
		characterSelected: false,
		charSelectionRow:  0,
	}

	// 1. สปินอัปเซิร์ฟเวอร์เครือข่ายจำลอง
	serverURL, err := StartMockServer()
	if err != nil {
		g.err = err
		return g
	}
	g.serverURL = serverURL

	// ตั้งเป้าพิกัดกล้องเริ่มต้น 3D ให้ Snap ทันทีที่ผู้เล่น (1,1)
	px := (1.0 - 5.5) * 20.0
	pz := (1.0 - 5.5) * 20.0
	g.camX = px - 30.0
	g.camY = -140.0
	g.camZ = pz - 150.0

	return g
}

// loadLevel โหลดด่านผ่านเครือข่ายจำลองพร้อม Snap พิกัดกล้อง 3D ทันที
func (g *Game) loadLevel(id int, startX, startY int) {
	if id < 1 {
		id = 1
	} else if id > 4 {
		id = 4
	}
	g.currentLevelID = id
	g.loading = true

	g.player.GridX = startX
	g.player.GridY = startY
	g.player.targetGridX = startX
	g.player.targetGridY = startY
	g.player.isMoving = false
	g.player.moveTicks = 0

	// กล้องดัก Snap เข้าตำแหน่ง 3D ผู้เล่นทันที ป้องกันแอนิเมชันสไลด์ข้ามแผนที่ยาว
	gridX, gridY := float64(startX), float64(startY)
	px := (gridX - 5.5) * 20.0
	pz := (gridY - 5.5) * 20.0
	g.camX = px - 30.0
	g.camY = -140.0
	g.camZ = pz - 150.0


	go func() {
		time.Sleep(600 * time.Millisecond)

		url := fmt.Sprintf("%s/level?id=%d", g.serverURL, id)
		fetchedLevel, err := FetchLevelFromNetwork(url)
		if err != nil {
			g.loading = false
			return
		}

		g.level = fetchedLevel
		g.loading = false
	}()
}

func (g *Game) selectCharacter(charType int) {
	g.player.CharType = charType
	g.player.Inventory = make([]ItemType, 0)
	if charType == 0 {
		g.player.MaxInventory = 5
		g.player.Inventory = append(g.player.Inventory, ItemEnergyChip)
	} else {
		g.player.MaxInventory = 6
		g.player.Inventory = append(g.player.Inventory, ItemBlueKey)
	}
	g.characterSelected = true
	g.loadLevel(1, 1, 1)
}

// Update อัปเดตสถานะเกม
func (g *Game) Update() error {
	// หากยังไม่ได้เลือกตัวละครหลัก
	if !g.characterSelected {
		mx, my := ebiten.CursorPosition()
		mouseClicked := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)

		// 1. ตรวจจับตำแหน่งเมาส์ในตัวเลือก
		for i := 0; i < 2; i++ {
			xMin, xMax := 30.0, 290.0
			yMin := float64(130 + i*80)
			yMax := float64(190 + i*80)

			if float64(mx) >= xMin && float64(mx) <= xMax && float64(my) >= yMin && float64(my) <= yMax {
				g.charSelectionRow = i
				if mouseClicked {
					g.selectCharacter(i)
				}
			}
		}

		// 2. ควบคุมด้วยคีย์บอร์ด
		if inpututil.IsKeyJustPressed(ebiten.KeyW) || inpututil.IsKeyJustPressed(ebiten.KeyUp) {
			g.charSelectionRow = (g.charSelectionRow - 1 + 2) % 2
		} else if inpututil.IsKeyJustPressed(ebiten.KeyS) || inpututil.IsKeyJustPressed(ebiten.KeyDown) {
			g.charSelectionRow = (g.charSelectionRow + 1) % 2
		}

		if inpututil.IsKeyJustPressed(ebiten.KeySpace) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			g.selectCharacter(g.charSelectionRow)
		}
		return nil
	}
	// เช็คการกดปุ่มเพื่อสั่ง Pause / Unpause / จัดการเมนู Settings
	if inpututil.IsKeyJustPressed(ebiten.KeyP) {
		if g.isPaused && g.simplePause {
			g.isPaused = false
		} else {
			g.isPaused = true
			g.simplePause = true
			g.settingsOpen = false
		}
	} else if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if g.isPaused {
			if g.settingsOpen {
				// ย้อนกลับจากเมนูตั้งค่าไปยังเมนูหลักแทนการยกเลิกหยุดเกม
				g.settingsOpen = false
			} else {
				g.isPaused = false
			}
		} else {
			g.isPaused = true
			g.simplePause = false
			g.pauseActiveRow = 0
			g.settingsOpen = false
		}
	}

	// ถ้าอยู่ในสถานะหยุดเกม (Pause)
	if g.isPaused {
		g.tick++
		if g.simplePause {
			// โหมด Simple Pause: ไม่รับการคีย์ควบคุมเมนูอื่นๆ
			return nil
		}

		// โหมด Menu Pause
		mx, my := ebiten.CursorPosition()
		mouseClicked := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)

		if g.settingsOpen {
			// 1. ตรวจสอบการโฮเวอร์และคลิกของเมาส์ในเมนู Settings (4 ตัวเลือก)
			for i := 0; i < 4; i++ {
				xMin := float64(60)
				xMax := float64(260)
				yMin := float64(130 + i*40)
				yMax := float64(158 + i*40)

				if float64(mx) >= xMin && float64(mx) <= xMax && float64(my) >= yMin && float64(my) <= yMax {
					g.settingActiveRow = i
					if mouseClicked {
						switch i {
						case 0: // Sound FX
							g.soundMuted = !g.soundMuted
						case 1: // CRT Filter
							g.retroFilter = !g.retroFilter
						case 2: // Camera Lerp speed
							if g.cameraSpeed == 0.08 {
								g.cameraSpeed = 0.15
							} else if g.cameraSpeed == 0.15 {
								g.cameraSpeed = 0.04
							} else {
								g.cameraSpeed = 0.08
							}
						case 3: // Back to Menu
							g.settingsOpen = false
						}
					}
				}
			}

			// ควบคุมด้วยคีย์บอร์ดในเมนู Settings
			if inpututil.IsKeyJustPressed(ebiten.KeyW) || inpututil.IsKeyJustPressed(ebiten.KeyUp) {
				g.settingActiveRow = (g.settingActiveRow - 1 + 4) % 4
			} else if inpututil.IsKeyJustPressed(ebiten.KeyS) || inpututil.IsKeyJustPressed(ebiten.KeyDown) {
				g.settingActiveRow = (g.settingActiveRow + 1) % 4
			}

			if inpututil.IsKeyJustPressed(ebiten.KeyA) || inpututil.IsKeyJustPressed(ebiten.KeyLeft) ||
				inpututil.IsKeyJustPressed(ebiten.KeyD) || inpututil.IsKeyJustPressed(ebiten.KeyRight) ||
				inpututil.IsKeyJustPressed(ebiten.KeySpace) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
				
				switch g.settingActiveRow {
				case 0: // Sound Effects (ON/OFF)
					g.soundMuted = !g.soundMuted
				case 1: // Retro Filter (ON/OFF)
					g.retroFilter = !g.retroFilter
				case 2: // Camera Speed (Slow / Normal / Fast)
					if g.cameraSpeed == 0.08 {
						g.cameraSpeed = 0.15 // Fast
					} else if g.cameraSpeed == 0.15 {
						g.cameraSpeed = 0.04 // Slow
					} else {
						g.cameraSpeed = 0.08 // Normal
					}
				case 3: // Back
					g.settingsOpen = false
				}
			}
		} else {
			// 2. ตรวจสอบการโฮเวอร์และคลิกของเมาส์ในเมนู Pause หลัก (4 ตัวเลือก)
			for i := 0; i < 4; i++ {
				xMin := float64(60)
				xMax := float64(260)
				yMin := float64(130 + i*40)
				yMax := float64(158 + i*40)

				if float64(mx) >= xMin && float64(mx) <= xMax && float64(my) >= yMin && float64(my) <= yMax {
					g.pauseActiveRow = i
					if mouseClicked {
						switch i {
						case 0: // Resume
							g.isPaused = false
						case 1: // Settings
							g.settingsOpen = true
							g.settingActiveRow = 0
						case 2: // Reset Floor
							g.loadLevel(g.currentLevelID, 1, 1)
							g.isPaused = false
						case 3: // Quit to Desktop
							os.Exit(0)
						}
					}
				}
			}

			// ควบคุมด้วยคีย์บอร์ดในเมนู Pause หลัก
			if inpututil.IsKeyJustPressed(ebiten.KeyW) || inpututil.IsKeyJustPressed(ebiten.KeyUp) {
				g.pauseActiveRow = (g.pauseActiveRow - 1 + 4) % 4
			} else if inpututil.IsKeyJustPressed(ebiten.KeyS) || inpututil.IsKeyJustPressed(ebiten.KeyDown) {
				g.pauseActiveRow = (g.pauseActiveRow + 1) % 4
			}

			if inpututil.IsKeyJustPressed(ebiten.KeySpace) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
				switch g.pauseActiveRow {
				case 0: // Resume
					g.isPaused = false
				case 1: // Settings
					g.settingsOpen = true
					g.settingActiveRow = 0
				case 2: // Reset Floor
					g.loadLevel(g.currentLevelID, 1, 1)
					g.isPaused = false
				case 3: // Quit to Desktop
					os.Exit(0)
				}
			}
		}
		return nil
	}

	if g.victory {
		g.tick++
		if ebiten.IsKeyPressed(ebiten.KeySpace) || ebiten.IsKeyPressed(ebiten.KeyEnter) {
			g.currentLevelID = 1
			g.victory = false
			g.player = NewPlayer(1, 1)
			g.loadLevel(1, 1, 1)
		}
		return nil
	}

	if g.loading {
		g.tick++
		return nil
	}

	// Performance Monitoring for FPS/TPS drops (วอร์มอัป 300 ticks หรือ 5 วินาทีแรก)
	if !g.loading && g.tick > 300 {
		currentFPS := ebiten.ActualFPS()
		currentTPS := ebiten.ActualTPS()
		if currentFPS < 55.0 || currentTPS < 55.0 {
			if time.Since(g.lastFPSReportTime) >= 5*time.Second {
				g.lastFPSReportTime = time.Now()
				go SendCategorizedReport(g.serverURL, "fps_drop", currentFPS, currentTPS, g.currentLevelID, fmt.Sprintf("FPS/TPS dropped below threshold (FPS: %.1f, TPS: %.1f)", currentFPS, currentTPS))
			}
		}
	}

	// 0. Modal Item Box Navigation
	if g.boxUIOpen {
		g.tick++
		if inpututil.IsKeyJustPressed(ebiten.KeyQ) {
			g.boxUIOpen = false
			return nil
		}

		if inpututil.IsKeyJustPressed(ebiten.KeyA) || inpututil.IsKeyJustPressed(ebiten.KeyLeft) {
			g.boxActiveCol = 0
		} else if inpututil.IsKeyJustPressed(ebiten.KeyD) || inpututil.IsKeyJustPressed(ebiten.KeyRight) {
			g.boxActiveCol = 1
		}

		cap := GetBoxCapacityForFloor(g.currentLevelID)
		if inpututil.IsKeyJustPressed(ebiten.KeyW) || inpututil.IsKeyJustPressed(ebiten.KeyUp) {
			if g.boxActiveCol == 0 {
				g.boxPlayerSlot = (g.boxPlayerSlot - 1 + g.player.MaxInventory) % g.player.MaxInventory
			} else if cap > 0 {
				g.boxItemSlot = (g.boxItemSlot - 1 + cap) % cap
			}
		} else if inpututil.IsKeyJustPressed(ebiten.KeyS) || inpututil.IsKeyJustPressed(ebiten.KeyDown) {
			if g.boxActiveCol == 0 {
				g.boxPlayerSlot = (g.boxPlayerSlot + 1) % g.player.MaxInventory
			} else if cap > 0 {
				g.boxItemSlot = (g.boxItemSlot + 1) % cap
			}
		}

		if inpututil.IsKeyJustPressed(ebiten.KeySpace) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			g.handleItemBoxTransfer()
		}

		return nil
	}

	// 1. เช็กสถานะการปีนบันไดแบบออโต้ล็อกสไตล์ RE0
	if g.player.isClimbing {
		g.player.Update(g.level)
		if g.player.climbTicks >= 45 {
			g.player.isClimbing = false
			g.player.climbTicks = 0

			if g.player.climbDirection == 1 {
				g.currentLevelID++
				g.loadLevel(g.currentLevelID, 1, 1)
			} else if g.player.climbDirection == -1 {
				g.currentLevelID--
				g.loadLevel(g.currentLevelID, 10, 10)
			}
		}
		g.updateCameraFollow()
		return nil
	}

	// Open Item Box UI if standing adjacent and pressing Space/Enter
	if !g.player.isMoving && g.IsPlayerAdjacentToBox() {
		if inpututil.IsKeyJustPressed(ebiten.KeySpace) || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			g.boxUIOpen = true
			g.boxActiveCol = 0
			g.boxPlayerSlot = 0
			g.boxItemSlot = 0
			return nil
		}
	}

	// 2. ปรับเปลี่ยนช่องสล็อตกระเป๋าเดินทาง HUD
	if inpututil.IsKeyJustPressed(ebiten.Key1) {
		g.selectedSlot = 0
	} else if inpututil.IsKeyJustPressed(ebiten.Key2) {
		g.selectedSlot = 1
	} else if inpututil.IsKeyJustPressed(ebiten.Key3) {
		g.selectedSlot = 2
	} else if inpututil.IsKeyJustPressed(ebiten.Key4) {
		g.selectedSlot = 3
	} else if inpututil.IsKeyJustPressed(ebiten.Key5) {
		g.selectedSlot = 4
	} else if inpututil.IsKeyJustPressed(ebiten.Key6) && g.player.MaxInventory >= 6 {
		g.selectedSlot = 5
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyTab) {
		g.selectedSlot = (g.selectedSlot + 1) % g.player.MaxInventory
	}

	// 3. ตรวจจับการทิ้งไอเทมลงพื้นตารางกริด [Q] หรือ [Backspace]
	if inpututil.IsKeyJustPressed(ebiten.KeyQ) || inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		g.player.DropItem(g.selectedSlot, g.level)
	}

	g.tick++
	g.player.Update(g.level)

	// 4. ตรวจจับการกดปีนเมื่อผู้เล่นยืนอยู่พิกัดตารางบันไดพอดี
	currentTile := g.level.Tiles[g.player.GridY][g.player.GridX]
	if !g.player.isMoving {
		if currentTile == TileStairsUp {
			if ebiten.IsKeyPressed(ebiten.KeySpace) || ebiten.IsKeyPressed(ebiten.KeyEnter) {
				g.player.StartClimb(1)
			}
		} else if currentTile == TileStairsDown {
			if ebiten.IsKeyPressed(ebiten.KeySpace) || ebiten.IsKeyPressed(ebiten.KeyEnter) {
				g.player.StartClimb(-1)
			}
		} else if currentTile == TileGoal {
			g.victory = true
		}
	}

	// 5. เลื่อนกล้องตามตัวละคร Gopher ในระบบพิกัด 3D
	g.updateCameraFollow()

	return nil
}

// updateCameraFollow อัปเดตพิกัดเลื่อนกล้องตามตำแหน่งผู้เล่นในโลก 3 มิติ
func (g *Game) updateCameraFollow() {
	gridX := g.player.GridX
	gridY := g.player.GridY

	var targetX, targetY, targetZ float64

	// แบ่งแผนที่ออกเป็น 4 โซนย่อยหลักตามตรรกะ Resident Evil 1 Remake Fixed Cameras
	if gridX < 6 && gridY < 6 {
		// โซน 1: พิกัดกล้องมุมสูงฝั่งเริ่มเกม (Spawn Area)
		targetX = 66.0
		targetY = -130.0
		targetZ = 66.0
	} else if gridX >= 6 && gridY < 6 {
		// โซน 2: พิกัดกล้องตรวจการทางขวาบน (Corridor East)
		targetX = 260.0
		targetY = -140.0
		targetZ = 60.0
	} else if gridX < 6 && gridY >= 6 {
		// โซน 3: พิกัดกล้องตรวจการทางซ้ายล่าง (Corridor West)
		targetX = 60.0
		targetY = -130.0
		targetZ = 260.0
	} else {
		// โซน 4: พิกัดกล้องมุมสูงฝั่งประตูทางออกและบันได (Exit & Stairs Area)
		targetX = 260.0
		targetY = -120.0
		targetZ = 260.0
	}

	// บังคับล็อคเป้าทันทีหากเพิ่งโหลดแผนที่ข้ามชั้นเพื่อไม่ให้เกิดการส่ายกล้องข้ามฉาก
	if g.lastZone == 0 {
		g.camX = targetX
		g.camY = targetY
		g.camZ = targetZ
		g.lastZone = 1
		return
	}

	// เคลื่อนกล้องด้วยอัตรา Lerp หาพิกัดล็อคในด่าน (PAN Transition) 
	// เพื่อคงความรู้สึกมุมกล้องภาพยนตร์ของ Resident Evil 1 Remake
	g.camX += (targetX - g.camX) * g.cameraSpeed
	g.camY += (targetY - g.camY) * g.cameraSpeed
	g.camZ += (targetZ - g.camZ) * g.cameraSpeed
}

// Draw แสดงผลหน้าจอเกมด้วย 3D Painter's Algorithm
func (g *Game) Draw(screen *ebiten.Image) {
	if !g.characterSelected {
		screen.Fill(color.RGBA{10, 10, 12, 255})

		// หัวเรื่อง
		title := "CHOOSE MAIN CHARACTER"
		titleX := float32(160 - (len(title)*6)/2)
		ebitenutil.DebugPrintAt(screen, title, int(titleX), 40)
		vector.StrokeLine(screen, 40, 60, 280, 60, 1.5, color.RGBA{197, 160, 89, 150}, false)

		// สองตัวเลือก: Gopher vs Rust
		rows := []struct {
			name  string
			desc  string
			desc2 string
			col   color.RGBA
		}{
			{
				name:  "GOPHER (Go Mascot)",
				desc:  "Starts with Energy Chip",
				desc2: "Capacity: 5 Slots",
				col:   color.RGBA{52, 152, 219, 255}, // Cyan
			},
			{
				name:  "RUST (Rust Mascot)",
				desc:  "Starts with Blue Key",
				desc2: "Capacity: 6 Slots (Extra space)",
				col:   color.RGBA{211, 84, 0, 255}, // Orange
			},
		}

		for i, char := range rows {
			btnY := float32(100 + i*90)
			btnW, btnH := float32(260), float32(70)
			btnX := float32(30)

			// เติมสีพื้นและเส้นกรอบเรืองแสงตามสถานะเลือก
			if i == g.charSelectionRow {
				vector.DrawFilledRect(screen, btnX, btnY, btnW, btnH, color.RGBA{char.col.R, char.col.G, char.col.B, 40}, false)
				vector.StrokeRect(screen, btnX, btnY, btnW, btnH, 2.0, char.col, false)
			} else {
				vector.DrawFilledRect(screen, btnX, btnY, btnW, btnH, color.RGBA{20, 20, 25, 200}, false)
				vector.StrokeRect(screen, btnX, btnY, btnW, btnH, 1.0, color.RGBA{50, 50, 55, 255}, false)
			}

			// พิมพ์ข้อความชื่อและสถิติ
			ebitenutil.DebugPrintAt(screen, "Character: "+char.name, int(btnX)+10, int(btnY)+10)
			ebitenutil.DebugPrintAt(screen, char.desc, int(btnX)+10, int(btnY)+30)
			ebitenutil.DebugPrintAt(screen, char.desc2, int(btnX)+10, int(btnY)+48)
		}

		helpText := "[W/S] Move Selection  [Space/Enter] Confirm"
		helpX := float32(160 - (len(helpText)*6)/2)
		ebitenutil.DebugPrintAt(screen, helpText, int(helpX), 300)
		return
	}

	if g.victory {
		screen.Fill(color.RGBA{15, 10, 20, 255})

		pulse := float32(math.Sin(float64(g.tick)*0.08) * 10)
		vector.DrawFilledCircle(screen, 160, 150, 40+pulse, color.RGBA{241, 196, 15, 40}, false)

		vector.DrawFilledRect(screen, 140, 185, 40, 10, color.RGBA{197, 160, 89, 255}, false)
		vector.DrawFilledRect(screen, 155, 165, 10, 20, color.RGBA{197, 160, 89, 255}, false)
		vector.DrawFilledCircle(screen, 160, 145, 20, color.RGBA{241, 196, 15, 255}, false)
		vector.DrawFilledRect(screen, 140, 125, 40, 20, color.RGBA{15, 10, 20, 255}, false)
		vector.DrawFilledRect(screen, 140, 135, 40, 15, color.RGBA{241, 196, 15, 255}, false)
		vector.StrokeCircle(screen, 137, 145, 10, 3, color.RGBA{197, 160, 89, 255}, false)
		vector.StrokeCircle(screen, 183, 145, 10, 3, color.RGBA{197, 160, 89, 255}, false)

		vector.DrawFilledRect(screen, 40, 230, 240, 30, color.RGBA{120, 20, 30, 255}, false)
		vector.StrokeRect(screen, 40, 230, 240, 30, 1.5, color.RGBA{197, 160, 89, 255}, false)

		skullCount := 0
		for _, item := range g.player.Inventory {
			if item == ItemBossSkull {
				skullCount++
			}
		}

		for i := 0; i < skullCount; i++ {
			x := float32(160 - (skullCount-1)*20 + i*40)
			y := float32(285)
			vector.DrawFilledCircle(screen, x, y, 8, color.RGBA{155, 89, 182, 255}, false)
			vector.DrawFilledRect(screen, x-3.5, y+5, 7, 3.5, color.RGBA{155, 89, 182, 255}, false)
			vector.DrawFilledCircle(screen, x-2.5, y-1, 1.5, color.RGBA{255, 255, 255, 255}, false)
			vector.DrawFilledCircle(screen, x+2.5, y-1, 1.5, color.RGBA{255, 255, 255, 255}, false)
		}

		return
	}

	if g.loading {
		screen.Fill(color.RGBA{10, 10, 12, 255})

		vector.DrawFilledRect(screen, 30, 100, 260, 100, color.RGBA{20, 20, 25, 255}, false)
		vector.StrokeRect(screen, 30, 100, 260, 100, 1, color.RGBA{52, 73, 94, 255}, false)

		barX := float32(60)
		barY := float32(150)
		barW := float32(200)
		barH := float32(12)

		vector.StrokeRect(screen, barX, barY, barW, barH, 1.5, color.RGBA{127, 140, 141, 180}, false)
		progressW := float32(math.Mod(float64(g.tick*3), float64(barW-6)))
		vector.DrawFilledRect(screen, barX+3, barY+3, progressW, barH-6, color.RGBA{26, 188, 156, 255}, false)
		return
	}

	screen.Fill(color.RGBA{10, 10, 12, 255})

	// 1. รวบรวมแผ่นภาพพื้นดิน กำแพง 3D และไอเทมของเลเวลทั้งหมด
	entities := g.level.GetRenderEntities(g.tick, g.camX, g.camY, g.camZ)

	// 2. บรรจุตัวละครผู้เล่น Gopher เข้าไปร่วมประเมิน Z-depth
	px, py, pz := g.player.Get3DPos()
	_, _, pDepth := Project3D(px, py, pz, g.camX, g.camY, g.camZ)
	
	entities = append(entities, RenderEntity{
		Depth: pDepth - 0.5, // ให้ลอยด้านหน้านิดหน่อยเพื่อความคมชัด
		Draw: func(screen *ebiten.Image) {
			g.player.Draw(screen, g.camX, g.camY, g.camZ)
		},
	})

	// 3. จัดเรียง Z-depth คัดสรรจากลึกสุด (Depth สูงสุด) ไปหาใกล้สุด (Depth ต่ำสุด) ด้วย sort.SliceStable
	sort.SliceStable(entities, func(i, j int) bool {
		return entities[i].Depth > entities[j].Depth
	})

	// 4. สั่งวาดทุกวัตถุตามระยะความลึกที่สมมาตรอย่างสมบูรณ์แบบ
	for _, entity := range entities {
		entity.Draw(screen)
	}

	// Apply Retro Scanline / CRT Vignette Filter if active
	if g.retroFilter {
		for y := 0; y < 320; y += 2 {
			vector.StrokeLine(screen, 0, float32(y), 320, float32(y), 1.0, color.RGBA{0, 0, 0, 45}, false)
		}
		// Vignette/Amber shadow
		vector.DrawFilledRect(screen, 0, 0, 320, 320, color.RGBA{243, 156, 18, 12}, false)
	}

	// 5. วาดป้ายแจ้งเตือนให้กด Space/Enter ปีนบันได หรือเปิด Item Box เมื่อตัวละครอยู่ในตำแหน่งเงื่อนไข
	if !g.player.isMoving && !g.player.isClimbing && !g.boxUIOpen {
		currentTile := g.level.Tiles[g.player.GridY][g.player.GridX]
		if currentTile == TileStairsUp || currentTile == TileStairsDown {
			vector.DrawFilledRect(screen, 60, 15, 200, 20, color.RGBA{0, 0, 0, 200}, false)
			vector.StrokeRect(screen, 60, 15, 200, 20, 1, color.RGBA{197, 160, 89, 255}, false)

			midX := float32(160)
			midY := float32(25)
			if currentTile == TileStairsUp {
				vector.DrawFilledCircle(screen, midX, midY-2, 3, color.RGBA{46, 204, 113, 255}, false)
				vector.DrawFilledRect(screen, midX-1, midY-2, 2, 6, color.RGBA{46, 204, 113, 255}, false)
			} else {
				vector.DrawFilledCircle(screen, midX, midY+2, 3, color.RGBA{231, 76, 60, 255}, false)
				vector.DrawFilledRect(screen, midX-1, midY-4, 2, 6, color.RGBA{231, 76, 60, 255}, false)
			}
		} else if g.IsPlayerAdjacentToBox() {
			vector.DrawFilledRect(screen, 40, 15, 240, 20, color.RGBA{0, 0, 0, 200}, false)
			vector.StrokeRect(screen, 40, 15, 240, 20, 1, color.RGBA{241, 196, 15, 255}, false)
			ebitenutil.DebugPrintAt(screen, "PRESS SPACE / ENTER TO OPEN ITEM BOX", 48, 18)
		}
	}

	// 6. วาดแผงสัมภาระ HUD สูง 48px ด้านล่างสุด (ตรึงพิกัดนิ่งถาวรที่ 320px)
	hudY := float32(320)
	hudHeight := float32(48)
	hudWidth := float32(320)

	vector.DrawFilledRect(screen, 0, hudY, hudWidth, hudHeight, color.RGBA{15, 15, 18, 255}, false)
	vector.StrokeLine(screen, 0, hudY, hudWidth, hudY, 2, color.RGBA{52, 73, 94, 255}, false)

	slotSize := float32(28)
	slotY := hudY + (hudHeight-slotSize)/2
	for i := 0; i < g.player.MaxInventory; i++ {
		slotX := float32(8 + i*34)
		vector.DrawFilledRect(screen, slotX, slotY, slotSize, slotSize, color.RGBA{25, 25, 30, 255}, false)

		if i == g.selectedSlot {
			vector.StrokeRect(screen, slotX-2, slotY-2, slotSize+4, slotSize+4, 2.0, color.RGBA{241, 196, 15, 255}, false)
		} else {
			vector.StrokeRect(screen, slotX, slotY, slotSize, slotSize, 1, color.RGBA{45, 45, 55, 255}, false)
		}

		if i < len(g.player.Inventory) {
			itemType := g.player.Inventory[i]
			itemX := slotX + slotSize/2
			itemY := slotY + slotSize/2

			switch itemType {
			case ItemRedKey:
				vector.DrawFilledCircle(screen, itemX, itemY-3, 4, color.RGBA{231, 76, 60, 255}, false)
				vector.DrawFilledRect(screen, itemX-1, itemY-1, 2, 8, color.RGBA{231, 76, 60, 255}, false)
				vector.DrawFilledRect(screen, itemX-1, itemY+3, 4, 1.5, color.RGBA{231, 76, 60, 255}, false)
				vector.DrawFilledRect(screen, itemX-1, itemY+5, 4, 1.5, color.RGBA{231, 76, 60, 255}, false)
			case ItemBlueKey:
				vector.DrawFilledCircle(screen, itemX, itemY-3, 4, color.RGBA{52, 152, 219, 255}, false)
				vector.DrawFilledRect(screen, itemX-1, itemY-1, 2, 8, color.RGBA{52, 152, 219, 255}, false)
				vector.DrawFilledRect(screen, itemX-1, itemY+3, 4, 1.5, color.RGBA{52, 152, 219, 255}, false)
				vector.DrawFilledRect(screen, itemX-1, itemY+5, 4, 1.5, color.RGBA{52, 152, 219, 255}, false)
			case ItemEnergyChip:
				vector.DrawFilledRect(screen, itemX-5, itemY-5, 10, 10, color.RGBA{241, 196, 15, 255}, false)
				vector.DrawFilledCircle(screen, itemX, itemY, 1.5, color.RGBA{255, 255, 255, 255}, false)
			case ItemBossSkull:
				vector.DrawFilledCircle(screen, itemX, itemY-1, 6, color.RGBA{155, 89, 182, 255}, false)
				vector.DrawFilledRect(screen, itemX-2.5, itemY+3.5, 5, 2.5, color.RGBA{155, 89, 182, 255}, false)
				vector.DrawFilledCircle(screen, itemX-2, itemY-2, 1.2, color.RGBA{255, 255, 255, 255}, false)
				vector.DrawFilledCircle(screen, itemX+2, itemY-2, 1.2, color.RGBA{255, 255, 255, 255}, false)
			}
		}
	}

	helperX := hudWidth - 85
	helperY := hudY + (hudHeight-24)/2
	
	// วาดกรอบสีเขียว/เหลือง/แดงตามสถานะชีพจร (ECG Frame)
	vector.DrawFilledRect(screen, helperX, helperY, 74, 24, color.RGBA{10, 10, 15, 255}, false)

	// คำนวณหาระยะอันตรายที่ใกล้ที่สุด (มินิบอส หรือ ประตูทางออกด่าน)
	playerX, playerY := g.player.GridX, g.player.GridY
	minDist := 999.0
	for y := 0; y < g.level.Height; y++ {
		for x := 0; x < g.level.Width; x++ {
			tile := g.level.Tiles[y][x]
			if tile == TileSecretBoss || tile == TileGoal {
				dist := math.Sqrt(float64((x-playerX)*(x-playerX) + (y-playerY)*(y-playerY)))
				if dist < minDist {
					minDist = dist
				}
			}
		}
	}

	panicLevel := 0.0
	if minDist <= 2.0 {
		panicLevel = 1.0
	} else if minDist >= 8.0 {
		panicLevel = 0.0
	} else {
		panicLevel = 1.0 - (minDist-2.0)/6.0
	}

	ecgColor := color.RGBA{46, 204, 113, 255} // Green (Fine)
	speedFactor := 0.08
	waveAmp := 4.0
	if panicLevel > 0.7 {
		ecgColor = color.RGBA{231, 76, 60, 255} // Red (Danger)
		speedFactor = 0.32
		waveAmp = 8.0
	} else if panicLevel > 0.3 {
		ecgColor = color.RGBA{241, 196, 15, 255} // Yellow (Caution)
		speedFactor = 0.16
		waveAmp = 6.0
	}

	vector.StrokeRect(screen, helperX, helperY, 74, 24, 1.0, ecgColor, false)

	// วาดคลื่นหัวใจ ECG กราฟิกจำลองวิ่งข้ามหน้าต่างตรวจจับสัญญาณชีพจร
	for i := 0; i < 68; i++ {
		phase := math.Mod(float64(g.tick)*speedFactor-float64(i)*0.18, 2.0*math.Pi)
		var val float64 = 0
		if phase > 0 && phase < 0.3 {
			val = math.Sin(phase*math.Pi/0.3) * 1.5
		} else if phase >= 0.3 && phase < 0.5 {
			val = -(phase - 0.3) / 0.2 * 3.0
		} else if phase >= 0.5 && phase < 0.7 {
			val = -3.0 + (phase-0.5)/0.2*13.0
		} else if phase >= 0.7 && phase < 0.9 {
			val = 10.0 - (phase-0.7)/0.2*14.0
		} else if phase >= 0.9 && phase < 1.2 {
			val = -4.0 + math.Sin((phase-0.9)*math.Pi/0.3)*2.0
		}
		
		yOffset := float32(val * (waveAmp / 6.0))
		if yOffset < -10 {
			yOffset = -10
		} else if yOffset > 10 {
			yOffset = 10
		}

		if i > 0 {
			prevPhase := math.Mod(float64(g.tick)*speedFactor-float64(i-1)*0.18, 2.0*math.Pi)
			var prevVal float64 = 0
			if prevPhase > 0 && prevPhase < 0.3 {
				prevVal = math.Sin(prevPhase*math.Pi/0.3) * 1.5
			} else if prevPhase >= 0.3 && prevPhase < 0.5 {
				prevVal = -(prevPhase - 0.3) / 0.2 * 3.0
			} else if prevPhase >= 0.5 && prevPhase < 0.7 {
				prevVal = -3.0 + (prevPhase-0.5)/0.2*13.0
			} else if prevPhase >= 0.7 && prevPhase < 0.9 {
				prevVal = 10.0 - (prevPhase-0.7)/0.2*14.0
			} else if prevPhase >= 0.9 && prevPhase < 1.2 {
				prevVal = -4.0 + math.Sin((prevPhase-0.9)*math.Pi/0.3)*2.0
			}
			prevYOffset := float32(prevVal * (waveAmp / 6.0))
			if prevYOffset < -10 {
				prevYOffset = -10
			} else if prevYOffset > 10 {
				prevYOffset = 10
			}

			vector.StrokeLine(screen, helperX+71-float32(i), helperY+12+prevYOffset, helperX+72-float32(i), helperY+12+yOffset, 1.0, ecgColor, false)
		}
	}

	// 7. วาด Performance overlay (FPS/TPS)
	fps := ebiten.ActualFPS()
	tps := ebiten.ActualTPS()
	vector.DrawFilledRect(screen, 6, 6, 110, 18, color.RGBA{10, 10, 15, 200}, false)
	borderColor := color.RGBA{46, 204, 113, 255}
	if fps < 60.0 || tps < 60.0 {
		borderColor = color.RGBA{231, 76, 60, 255}
	}
	vector.StrokeRect(screen, 6, 6, 110, 18, 1.0, borderColor, false)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("FPS:%.1f TPS:%.1f", fps, tps), 10, 8)

	// 8. แสดงผล Split HUD Modal เมื่อเปิดใช้งาน Item Box
	if g.boxUIOpen {
		g.drawBoxUI(screen)
	}

	// 9. แสดงผล Pause Overlay เมื่อกดหยุดเกม
	if g.isPaused {
		// ทับเงาสลัวทั้งจอภาพเสมือน 320x368
		vector.DrawFilledRect(screen, 0, 0, 320, 368, color.RGBA{0, 0, 0, 200}, false)

		if g.simplePause {
			// โหมด Simple Pause (ปุ่ม P)
			panelW, panelH := float32(180), float32(60)
			panelX, panelY := float32(160-panelW/2), float32(160-panelH/2)
			
			// วาดกรอบสว่างสไตล์ยุคเก่า
			vector.DrawFilledRect(screen, panelX, panelY, panelW, panelH, color.RGBA{20, 10, 10, 255}, false)
			vector.StrokeRect(screen, panelX, panelY, panelW, panelH, 2.0, color.RGBA{231, 76, 60, 255}, false)
			
			ebitenutil.DebugPrintAt(screen, "GAME PAUSED", int(panelX)+50, int(panelY)+15)
			ebitenutil.DebugPrintAt(screen, "Press P to Resume", int(panelX)+34, int(panelY)+35)
		} else {
			// โหมด Menu Pause (ปุ่ม Esc)
			if g.settingsOpen {
				// 1. หน้าจอเมนูตั้งค่า Settings
				titleText := "S E T T I N G S"
				titleX := float32(160 - (len(titleText)*6)/2)
				ebitenutil.DebugPrintAt(screen, titleText, int(titleX), 80)
				vector.StrokeLine(screen, 80, 102, 240, 102, 1.5, color.RGBA{197, 160, 89, 150}, false)

				soundLabel := "ON"
				if g.soundMuted {
					soundLabel = "OFF"
				}
				filterLabel := "OFF"
				if g.retroFilter {
					filterLabel = "CRT"
				}
				speedLabel := "Normal"
				if g.cameraSpeed == 0.04 {
					speedLabel = "Slow"
				} else if g.cameraSpeed == 0.15 {
					speedLabel = "Fast"
				}

				rows := []string{
					fmt.Sprintf("Sound FX: [%s]", soundLabel),
					fmt.Sprintf("CRT Filter: [%s]", filterLabel),
					fmt.Sprintf("Camera Lerp: [%s]", speedLabel),
					"Back to Menu",
				}

				btnW, btnH := float32(200), float32(26)
				btnX := float32(60)

				for i, rowText := range rows {
					btnY := float32(130 + i*40)
					textX := btnX + (btnW-float32(len(rowText)*6))/2
					textY := btnY + (btnH-12)/2

					if i == g.settingActiveRow {
						// ปุ่มที่กำลังถูกเลือก: ไฮไลต์สีทองสว่าง
						vector.DrawFilledRect(screen, btnX, btnY, btnW, btnH, color.RGBA{197, 160, 89, 70}, false)
						vector.StrokeRect(screen, btnX, btnY, btnW, btnH, 1.5, color.RGBA{241, 196, 15, 255}, false)
						ebitenutil.DebugPrintAt(screen, rowText, int(textX), int(textY))
					} else {
						// ปุ่มปกติ: กรอบสีเข้มโปร่งแสง
						vector.DrawFilledRect(screen, btnX, btnY, btnW, btnH, color.RGBA{25, 25, 30, 150}, false)
						vector.StrokeRect(screen, btnX, btnY, btnW, btnH, 1.0, color.RGBA{70, 70, 75, 255}, false)
						ebitenutil.DebugPrintAt(screen, rowText, int(textX), int(textY))
					}
				}

				helpText := "[W/S] Navigate  [Space/Enter/A/D] Toggle"
				helpX := float32(160 - (len(helpText)*6)/2)
				ebitenutil.DebugPrintAt(screen, helpText, int(helpX), 300)
			} else {
				// 2. หน้าจอเมนูหยุดเกมหลัก Pause Menu
				titleText := "P A U S E D"
				titleX := float32(160 - (len(titleText)*6)/2)
				ebitenutil.DebugPrintAt(screen, titleText, int(titleX), 80)
				vector.StrokeLine(screen, 80, 102, 240, 102, 1.5, color.RGBA{197, 160, 89, 150}, false)

				rows := []string{
					"Resume Game",
					"Settings",
					"Restart Floor",
					"Quit to Desktop",
				}

				btnW, btnH := float32(200), float32(26)
				btnX := float32(60)

				for i, rowText := range rows {
					btnY := float32(130 + i*40)
					textX := btnX + (btnW-float32(len(rowText)*6))/2
					textY := btnY + (btnH-12)/2

					if i == g.pauseActiveRow {
						// ปุ่มที่กำลังถูกเลือก: ไฮไลต์สีทองสว่าง
						vector.DrawFilledRect(screen, btnX, btnY, btnW, btnH, color.RGBA{197, 160, 89, 70}, false)
						vector.StrokeRect(screen, btnX, btnY, btnW, btnH, 1.5, color.RGBA{241, 196, 15, 255}, false)
						ebitenutil.DebugPrintAt(screen, rowText, int(textX), int(textY))
					} else {
						// ปุ่มปกติ: กรอบสีเข้มโปร่งแสง
						vector.DrawFilledRect(screen, btnX, btnY, btnW, btnH, color.RGBA{25, 25, 30, 150}, false)
						vector.StrokeRect(screen, btnX, btnY, btnW, btnH, 1.0, color.RGBA{70, 70, 75, 255}, false)
						ebitenutil.DebugPrintAt(screen, rowText, int(textX), int(textY))
					}
				}

				helpText := "[W/S] Navigate  [Space/Enter] Select"
				helpX := float32(160 - (len(helpText)*6)/2)
				ebitenutil.DebugPrintAt(screen, helpText, int(helpX), 300)
			}
		}
	}
}

func GetBoxCapacityForFloor(levelID int) int {
	switch levelID {
	case 1:
		return 6
	case 2:
		return 4
	case 3:
		return 3
	case 4:
		return 2
	default:
		return 6
	}
}

func (g *Game) GetCurrentBoxInventory() []ItemType {
	if g.boxInventories == nil {
		g.boxInventories = make(map[int][]ItemType)
	}
	return g.boxInventories[g.currentLevelID]
}

func (g *Game) SetCurrentBoxInventory(inv []ItemType) {
	if g.boxInventories == nil {
		g.boxInventories = make(map[int][]ItemType)
	}
	g.boxInventories[g.currentLevelID] = inv
}

func (g *Game) IsPlayerAdjacentToBox() bool {
	if g.level == nil || g.player == nil {
		return false
	}
	if g.player.isMoving || g.player.isClimbing {
		return false
	}
	dirs := [][2]int{{0, -1}, {0, 1}, {-1, 0}, {1, 0}}
	for _, d := range dirs {
		nx := g.player.GridX + d[0]
		ny := g.player.GridY + d[1]
		if nx >= 0 && nx < g.level.Width && ny >= 0 && ny < g.level.Height {
			if g.level.Tiles[ny][nx] == TileItemBox {
				return true
			}
		}
	}
	return false
}

func (g *Game) handleItemBoxTransfer() {
	playerInv := g.player.Inventory
	boxInv := g.GetCurrentBoxInventory()
	cap := GetBoxCapacityForFloor(g.currentLevelID)

	pIdx := g.boxPlayerSlot
	bIdx := g.boxItemSlot

	pHasItem := pIdx < len(playerInv)
	bHasItem := bIdx < len(boxInv)

	if g.boxActiveCol == 0 { // Initiated from Player side
		if !pHasItem {
			return
		}
		pItem := playerInv[pIdx]

		if bHasItem {
			// Direct Swap
			playerInv[pIdx], boxInv[bIdx] = boxInv[bIdx], pItem
		} else {
			// Transfer Player -> Box if capacity permits
			if len(boxInv) < cap {
				boxInv = append(boxInv, pItem)
				playerInv = append(playerInv[:pIdx], playerInv[pIdx+1:]...)
			}
		}
	} else { // Initiated from Box side
		if !bHasItem {
			return
		}
		bItem := boxInv[bIdx]

		if pHasItem {
			// Direct Swap
			playerInv[pIdx], boxInv[bIdx] = bItem, playerInv[pIdx]
		} else {
			// Transfer Box -> Player if player capacity permits (< 5)
			if len(playerInv) < 5 {
				playerInv = append(playerInv, bItem)
				boxInv = append(boxInv[:bIdx], boxInv[bIdx+1:]...)
			}
		}
	}

	g.player.Inventory = playerInv
	g.SetCurrentBoxInventory(boxInv)
}

func (g *Game) drawBoxUI(screen *ebiten.Image) {
	// Dark modal background overlay
	vector.DrawFilledRect(screen, 10, 20, 300, 290, color.RGBA{15, 15, 22, 245}, false)
	vector.StrokeRect(screen, 10, 20, 300, 290, 2.0, color.RGBA{197, 160, 89, 255}, false)

	cap := GetBoxCapacityForFloor(g.currentLevelID)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("STORAGE ITEM BOX (LVL %d)", g.currentLevelID), 75, 26)
	vector.StrokeLine(screen, 160, 42, 160, 275, 1.5, color.RGBA{52, 73, 94, 255}, false)

	// Left Column: Player Inventory (MaxInventory slots)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("PLAYER (%d)", g.player.MaxInventory), 45, 45)
	playerInv := g.player.Inventory
	for i := 0; i < g.player.MaxInventory; i++ {
		slotY := float32(65 + i*40)
		slotX := float32(20)
		slotW := float32(130)
		slotH := float32(34)

		vector.DrawFilledRect(screen, slotX, slotY, slotW, slotH, color.RGBA{25, 25, 32, 255}, false)

		if g.boxActiveCol == 0 && i == g.boxPlayerSlot {
			vector.StrokeRect(screen, slotX, slotY, slotW, slotH, 2.0, color.RGBA{241, 196, 15, 255}, false)
		} else {
			vector.StrokeRect(screen, slotX, slotY, slotW, slotH, 1.0, color.RGBA{50, 50, 60, 255}, false)
		}

		if i < len(playerInv) {
			item := playerInv[i]
			g.drawItemSlotIcon(screen, item, slotX+15, slotY+17)
			ebitenutil.DebugPrintAt(screen, g.getItemName(item), int(slotX)+32, int(slotY)+10)
		} else {
			ebitenutil.DebugPrintAt(screen, "-- EMPTY --", int(slotX)+32, int(slotY)+10)
		}
	}

	// Right Column: Box Inventory (N slots capacity)
	ebitenutil.DebugPrintAt(screen, fmt.Sprintf("BOX (%d)", cap), 200, 45)
	boxInv := g.GetCurrentBoxInventory()
	for i := 0; i < cap; i++ {
		slotY := float32(65 + i*40)
		slotX := float32(170)
		slotW := float32(130)
		slotH := float32(34)

		vector.DrawFilledRect(screen, slotX, slotY, slotW, slotH, color.RGBA{25, 25, 32, 255}, false)

		if g.boxActiveCol == 1 && i == g.boxItemSlot {
			vector.StrokeRect(screen, slotX, slotY, slotW, slotH, 2.0, color.RGBA{241, 196, 15, 255}, false)
		} else {
			vector.StrokeRect(screen, slotX, slotY, slotW, slotH, 1.0, color.RGBA{50, 50, 60, 255}, false)
		}

		if i < len(boxInv) {
			item := boxInv[i]
			g.drawItemSlotIcon(screen, item, slotX+15, slotY+17)
			ebitenutil.DebugPrintAt(screen, g.getItemName(item), int(slotX)+32, int(slotY)+10)
		} else {
			ebitenutil.DebugPrintAt(screen, "-- EMPTY --", int(slotX)+32, int(slotY)+10)
		}
	}

	// Footer controls legend
	vector.DrawFilledRect(screen, 15, 280, 290, 24, color.RGBA{20, 20, 28, 255}, false)
	vector.StrokeRect(screen, 15, 280, 290, 24, 1.0, color.RGBA{80, 80, 100, 255}, false)
	ebitenutil.DebugPrintAt(screen, "[A/D]Col [W/S]Slot [Enter]Swap [Q]Close", 22, 285)
}

func (g *Game) getItemName(item ItemType) string {
	switch item {
	case ItemRedKey:
		return "Red Key"
	case ItemBlueKey:
		return "Blue Key"
	case ItemEnergyChip:
		return "Chip"
	case ItemBossSkull:
		return "Skull"
	default:
		return "Item"
	}
}

func (g *Game) drawItemSlotIcon(screen *ebiten.Image, itemType ItemType, itemX, itemY float32) {
	switch itemType {
	case ItemRedKey:
		vector.DrawFilledCircle(screen, itemX, itemY-3, 3, color.RGBA{231, 76, 60, 255}, false)
		vector.DrawFilledRect(screen, itemX-1, itemY-1, 2, 6, color.RGBA{231, 76, 60, 255}, false)
	case ItemBlueKey:
		vector.DrawFilledCircle(screen, itemX, itemY-3, 3, color.RGBA{52, 152, 219, 255}, false)
		vector.DrawFilledRect(screen, itemX-1, itemY-1, 2, 6, color.RGBA{52, 152, 219, 255}, false)
	case ItemEnergyChip:
		vector.DrawFilledRect(screen, itemX-4, itemY-4, 8, 8, color.RGBA{241, 196, 15, 255}, false)
	case ItemBossSkull:
		vector.DrawFilledCircle(screen, itemX, itemY-1, 5, color.RGBA{155, 89, 182, 255}, false)
	}
}

// Layout กำหนดขนาดหน้าจอเสมือน
func (g *Game) Layout(outsideWidth, outsideHeight int) (screenWidth, screenHeight int) {
	return 320, 368
}
