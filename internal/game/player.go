package game

import (
	"bytes"
	"image"
	"image/color"
	_ "image/png"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/soulchip/game/assets"
)

type Player struct {
	GridX int
	GridY int

	targetGridX int
	targetGridY int

	isMoving  bool
	moveTicks int
	maxTicks  int

	inputDx int
	inputDy int

	// ระบบขึ้นลงบันได RE0
	isClimbing     bool
	climbTicks     int
	climbDirection int

	Inventory []ItemType
	sprite    *ebiten.Image
}

func (p *Player) SetInput(dx, dy int) {
	p.inputDx = dx
	p.inputDy = dy
}

func NewPlayer(startX, startY int) *Player {
	var sprite *ebiten.Image
	img, _, err := image.Decode(bytes.NewReader(assets.PlayerPNG))
	if err == nil {
		sprite = ebiten.NewImageFromImage(img)
	}

	return &Player{
		GridX:          startX,
		GridY:          startY,
		targetGridX:    startX,
		targetGridY:    startY,
		isMoving:       false,
		moveTicks:      0,
		maxTicks:       10,
		isClimbing:     false,
		climbTicks:     0,
		climbDirection: 0,
		Inventory:      make([]ItemType, 0),
		sprite:         sprite,
	}
}

func (p *Player) StartClimb(direction int) {
	p.isClimbing = true
	p.climbTicks = 0
	p.climbDirection = direction
}

// Update ประมวลผลและควบคุมผู้เล่น พร้อมตรวจสิทธิและพิกัดการไขกุญแจและปราบมินิบอส
func (p *Player) Update(level *Level) {
	if p.isClimbing {
		p.climbTicks++
		return
	}

	if p.isMoving {
		p.moveTicks++
		if p.moveTicks >= p.maxTicks {
			p.GridX = p.targetGridX
			p.GridY = p.targetGridY
			p.isMoving = false
			p.moveTicks = 0

			// 1. ปราบมินิบอสลับเมื่อก้าวขยับทับตำแหน่ง
			if level.Tiles[p.GridY][p.GridX] == TileSecretBoss {
				level.Tiles[p.GridY][p.GridX] = TileEmpty
				p.Inventory = append(p.Inventory, ItemBossSkull)
			}

			// 2. เก็บสะสมไอเทมคีย์และชิปในด่าน
			for i := 0; i < len(level.Items); i++ {
				item := &level.Items[i]
				if !item.Collected && item.GridX == p.GridX && item.GridY == p.GridY {
					item.Collected = true
					p.Inventory = append(p.Inventory, item.Type)
				}
			}
		}
	}

	if !p.isMoving {
		dx, dy := 0, 0
		if p.inputDx != 0 || p.inputDy != 0 {
			dx, dy = p.inputDx, p.inputDy
			p.inputDx = 0
			p.inputDy = 0
		} else {
			// รองรับการป้อนข้อมูลแนวเฉียง (Diagonal Inputs)
			if ebiten.IsKeyPressed(ebiten.KeyArrowUp) || ebiten.IsKeyPressed(ebiten.KeyW) {
				dy = -1
			} else if ebiten.IsKeyPressed(ebiten.KeyArrowDown) || ebiten.IsKeyPressed(ebiten.KeyS) {
				dy = 1
			}

			if ebiten.IsKeyPressed(ebiten.KeyArrowLeft) || ebiten.IsKeyPressed(ebiten.KeyA) {
				dx = -1
			} else if ebiten.IsKeyPressed(ebiten.KeyArrowRight) || ebiten.IsKeyPressed(ebiten.KeyD) {
				dx = 1
			}
		}

		if dx != 0 || dy != 0 {
			newX := p.GridX + dx
			newY := p.GridY + dy

			// ระบบวิเคราะห์ทิศทางและชนสไลด์กำแพง (Sliding Collision Resolution)
			if dx != 0 && dy != 0 {
				// ตรวจสอบว่าช่องปลายทางเฉียงติดกำแพงหรือไม่
				if level.IsWall(newX, newY) {
					// ลองสไลด์แนวระนาบก่อน (Horizontal slide)
					if !level.IsWall(p.GridX+dx, p.GridY) {
						newX = p.GridX + dx
						newY = p.GridY
						dy = 0
					} else if !level.IsWall(p.GridX, p.GridY+dy) { // ลองสไลด์แนวดิ่ง (Vertical slide)
						newX = p.GridX
						newY = p.GridY + dy
						dx = 0
					} else {
						// ไปต่อไม่ได้ทั้งสองแกน
						dx = 0
						dy = 0
					}
				}
			}

			// ถ้าทิศทางสุทธิที่สไลด์แล้วยังมีผลให้เคลื่อนที่ได้
			if dx != 0 || dy != 0 {
				// ตรวจสอบระบบไขประตูแดง/น้ำเงินด้วยไอเทมในสัมภาระ
				if newX >= 0 && newX < level.Width && newY >= 0 && newY < level.Height {
					tile := level.Tiles[newY][newX]
					if tile == TileRedDoor {
						if p.hasItem(ItemRedKey) {
							p.removeItem(ItemRedKey)
							level.Tiles[newY][newX] = TileEmpty
						}
					} else if tile == TileBlueDoor {
						if p.hasItem(ItemBlueKey) {
							p.removeItem(ItemBlueKey)
							level.Tiles[newY][newX] = TileEmpty
						}
					}
				}

				// บังคับเดินถ้าช่องปลายทางเป็นพื้นว่างหรือบันได (IsWall คืนค่า false สำหรับบันได)
				if !level.IsWall(newX, newY) {
					p.isMoving = true
					p.targetGridX = newX
					p.targetGridY = newY
					p.moveTicks = 1

					if ebiten.IsKeyPressed(ebiten.KeyShiftLeft) {
						p.maxTicks = 5
					} else {
						p.maxTicks = 10
					}
				}
			}
		}
	}
}

// Get3DPos คืนค่าตำแหน่งพิกัด X, Y, Z สามมิติจริงในอวกาศโลกของเกมสำหรับระบบกล้อง
func (p *Player) Get3DPos() (float64, float64, float64) {
	var gridX, gridY float64

	if p.isClimbing {
		t := float64(p.climbTicks) / 45.0
		dir := float64(p.climbDirection)
		// เลื่อนเฉียงขึ้นบันได 3D
		gridX = float64(p.GridX) + t*dir
		gridY = float64(p.GridY) + t*dir
	} else if p.isMoving {
		t := float64(p.moveTicks) / float64(p.maxTicks)
		gridX = float64(p.GridX) + (float64(p.targetGridX)-float64(p.GridX))*t
		gridY = float64(p.GridY) + (float64(p.targetGridY)-float64(p.GridY))*t
	} else {
		gridX = float64(p.GridX)
		gridY = float64(p.GridY)
	}

	x := (gridX - 5.5) * 20.0
	z := (gridY - 5.5) * 20.0

	y := 0.0
	if p.isClimbing {
		t := float64(p.climbTicks) / 45.0
		dir := float64(p.climbDirection)
		// สูงการปัดปีนแนวดิ่ง
		y = -t * 24.0 * dir
	}

	return x, y, z
}

// DropItem ทิ้งไอเทมจากสล็อตเป้ลงพิกัด Isometric พื้นตารางของแผนที่ตามสไตล์ RE0
func (p *Player) DropItem(slotIdx int, level *Level) bool {
	if p.isMoving || p.isClimbing {
		return false
	}
	if slotIdx < 0 || slotIdx >= len(p.Inventory) {
		return false
	}

	tile := level.Tiles[p.GridY][p.GridX]
	if tile == TileStairsUp || tile == TileStairsDown {
		return false
	}

	for _, item := range level.Items {
		if !item.Collected && item.GridX == p.GridX && item.GridY == p.GridY {
			return false
		}
	}

	itemType := p.Inventory[slotIdx]
	p.Inventory = append(p.Inventory[:slotIdx], p.Inventory[slotIdx+1:]...)

	level.Items = append(level.Items, Item{
		GridX:     p.GridX,
		GridY:     p.GridY,
		Type:      itemType,
		Collected: false,
	})

	return true
}

func (p *Player) hasItem(t ItemType) bool {
	for _, item := range p.Inventory {
		if item == t {
			return true
		}
	}
	return false
}

func (p *Player) removeItem(t ItemType) {
	for i, item := range p.Inventory {
		if item == t {
			p.Inventory = append(p.Inventory[:i], p.Inventory[i+1:]...)
			return
		}
	}
}

// Draw วาด Gopher ในระบบพิกัด 3D Perspective สเกลย่อขยายตัวละครอัตโนมัติตามค่าลึกความไกลจากกล้อง
func (p *Player) Draw(screen *ebiten.Image, camX, camY, camZ float64) {
	x, y, z := p.Get3DPos()
	projX, projY, depth := Project3D(x, y, z, camX, camY, camZ)

	// แอนิเมชันกระโดด/ปีนบันไดแนวดิ่ง (Sin-wave Bobbing เฉพาะ Draw)
	var bobbing float32
	if p.isClimbing {
		t := float64(p.climbTicks) / 45.0
		stepBob := math.Sin(t*math.Pi*8.0) * -2.0
		bobbing = float32(stepBob)
	} else if p.isMoving {
		t := float64(p.moveTicks) / float64(p.maxTicks)
		bobValue := math.Sin(math.Pi * t) * -4.0
		bobbing = float32(bobValue)
	}
	projY += bobbing

	// สเกลตามระยะห่าง (Perspective Scaling)
	fScale := float64(240.0 / depth)
	if fScale > 2.0 {
		fScale = 2.0
	}

	if p.sprite != nil {
		w, h := p.sprite.Bounds().Dx(), p.sprite.Bounds().Dy()
		op := &ebiten.DrawImageOptions{}

		targetW := 18.0 * fScale
		targetH := targetW * (float64(h) / float64(w))
		scaleX := targetW / float64(w)
		scaleY := targetH / float64(h)
		op.GeoM.Scale(scaleX, scaleY)

		offsetX := -targetW / 2
		offsetY := -targetH + 4*fScale
		op.GeoM.Translate(float64(projX)+offsetX, float64(projY)+offsetY)

		screen.DrawImage(p.sprite, op)
	} else {
		vector.DrawFilledCircle(screen, projX, projY, 4*float32(fScale), color.RGBA{0, 0, 0, 100}, false)
		vector.DrawFilledCircle(screen, projX, projY-6*float32(fScale), 5*float32(fScale), color.RGBA{230, 126, 34, 255}, false)
	}
}
