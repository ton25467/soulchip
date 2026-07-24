package game

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type TileType int

const (
	TileEmpty TileType = iota
	TileWall
	TileSwitch
	TileGoal
	TileRedDoor
	TileBlueDoor
	TileSecretBoss
	TileFakeWall
	TileStairsUp
	TileStairsDown
	TileItemBox
)

const (
	TileSize = 32
)

type ItemType int

const (
	ItemNone ItemType = iota
	ItemRedKey
	ItemBlueKey
	ItemEnergyChip
	ItemBossSkull
)

type Item struct {
	GridX     int      `json:"x"`
	GridY     int      `json:"y"`
	Type      ItemType `json:"type"`
	Collected bool     `json:"-"`
}

type Level struct {
	ID     int          `json:"id"`
	Width  int          `json:"width"`
	Height int          `json:"height"`
	Tiles  [][]TileType `json:"tiles"`
	Items  []Item       `json:"items"`
}

type RenderEntity struct {
	Depth float64
	Draw  func(screen *ebiten.Image)
}

func NewDefaultLevel() *Level {
	layout := [][]TileType{
		{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
		{1, 0, 0, 10, 1, 0, 0, 0, 0, 0, 0, 1},
		{1, 0, 1, 0, 1, 0, 1, 1, 1, 1, 0, 1},
		{1, 0, 1, 0, 0, 0, 0, 0, 0, 1, 0, 1},
		{1, 0, 1, 1, 1, 1, 0, 1, 0, 1, 0, 1},
		{1, 0, 0, 0, 0, 1, 0, 1, 0, 1, 0, 1},
		{1, 1, 1, 1, 0, 1, 0, 1, 0, 1, 0, 1},
		{1, 0, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1},
		{1, 0, 0, 1, 4, 1, 0, 1, 0, 1, 0, 1}, // (4,8) ประตูแดง
		{1, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0, 1},
		{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 8, 1}, // (10,10) บันไดขึ้นชั้น 2
		{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
	}
	return &Level{
		ID:     1,
		Width:  len(layout[0]),
		Height: len(layout),
		Tiles:  layout,
		Items: []Item{
			{GridX: 10, GridY: 1, Type: ItemRedKey},
		},
	}
}

func (l *Level) IsWall(gridX, gridY int) bool {
	if gridX < 0 || gridX >= l.Width || gridY < 0 || gridY >= l.Height {
		return true
	}
	t := l.Tiles[gridY][gridX]
	return t == TileWall || t == TileFakeWall || t == TileRedDoor || t == TileBlueDoor || t == TileItemBox
}

// -------------------------------------------------------------
// ระบบพิกัด 3D Perspective Software Engine
// -------------------------------------------------------------

const (
	BlockHeight = 24.0
)

// Project3D คำนวณฉายพิกัด 3 มิติตามมุมกล้อง Yaw & Pitch และแปลงขนาด Perspective
func Project3D(x, y, z float64, camX, camY, camZ float64) (float32, float32, float64) {
	// 1. หมุนย้ายเทียบตำแหน่งจุดศูนย์กลางกล้อง
	dx := x - camX
	dy := y - camY
	dz := z - camZ

	// 2. หมุนกล้องแกน Y (Yaw = 45 องศาเพื่อมุมเฉลียง)
	cosY, sinY := math.Cos(math.Pi/4), math.Sin(math.Pi/4)
	x1 := dx*cosY - dz*sinY
	z1 := dx*sinY + dz*cosY

	// 3. ก้มหัวกล้องลงมองต่ำรอบแกน X (Pitch = 35 องศา)
	cosX, sinX := math.Cos(math.Pi/5.14), math.Sin(math.Pi/5.14)
	y2 := dy*cosX - z1*sinX
	z2 := dy*sinX + z1*cosX

	// 4. Perspective Division
	focalLength := 240.0
	if z2 <= 5.0 {
		z2 = 5.0
	}
	projX := (x1 / z2) * focalLength + 160.0
	projY := (y2 / z2) * focalLength + 160.0

	return float32(projX), float32(projY), z2
}

type pointF struct {
	x, y float32
}

var whiteImage = ebiten.NewImage(1, 1)

func init() {
	whiteImage.Fill(color.White)
}

func drawIsoPolygon(screen *ebiten.Image, pts []pointF, c color.Color) {
	if len(pts) < 3 {
		return
	}
	var path vector.Path
	path.MoveTo(pts[0].x, pts[0].y)
	for i := 1; i < len(pts); i++ {
		path.LineTo(pts[i].x, pts[i].y)
	}
	path.Close()

	vertices, indices := path.AppendVerticesAndIndicesForFilling(nil, nil)

	r, g, b, a := c.RGBA()
	rf := float32(r) / 0xffff
	gf := float32(g) / 0xffff
	bf := float32(b) / 0xffff
	af := float32(a) / 0xffff

	for i := range vertices {
		vertices[i].SrcX = 0
		vertices[i].SrcY = 0
		vertices[i].ColorR = rf
		vertices[i].ColorG = gf
		vertices[i].ColorB = bf
		vertices[i].ColorA = af
	}

	screen.DrawTriangles(vertices, indices, whiteImage, nil)
}

// GetRenderEntities รวบรวมแผ่นภาพพื้นดิน บันได กำแพง 3D และไอเทมทั้งหมดเพื่อนำไปคำนวณ Z-sorting (Painter's Algorithm)
func (l *Level) GetRenderEntities(tick int, camX, camY, camZ float64) []RenderEntity {
	var entities []RenderEntity

	for y := 0; y < l.Height; y++ {
		for x := 0; x < l.Width; x++ {
			tile := l.Tiles[y][x]
			gridX, gridY := float64(x), float64(y)

			// ศูนย์กลางตรรกะในโลก 3 มิติของแผ่นกระเบื้อง
			cx := (gridX - 5.5) * 20.0
			cy := 0.0
			cz := (gridY - 5.5) * 20.0
			_, _, depth := Project3D(cx, cy, cz, camX, camY, camZ)

			localX, localY := x, y

			// 1. แผ่นภาพพื้นดิน 3D
			entities = append(entities, RenderEntity{
				Depth: depth + 3.0, // วาดไว้หลังสุดสำหรับแผ่นช่องนั้น
				Draw: func(screen *ebiten.Image) {
					l.draw3DFloor(screen, localX, localY, camX, camY, camZ)
				},
			})

			// 2. เตาปฏิกรณ์เป้าหมายด่าน 4
			if tile == TileGoal {
				entities = append(entities, RenderEntity{
					Depth: depth + 2.5,
					Draw: func(screen *ebiten.Image) {
						l.draw3DGoal(screen, localX, localY, camX, camY, camZ)
					},
				})
			}

			// 3. เสากำแพง 3D ลูกบาศก์ ประตูล็อก มินิบอส กล่องเก็บของ
			if tile == TileWall || tile == TileFakeWall || tile == TileRedDoor || tile == TileBlueDoor || tile == TileSecretBoss || tile == TileItemBox {
				// ความสูงจุดศูนย์กลางของลูกบาศก์ผนัง (ลอยขึ้นครึ่งหนึ่งคือ Y=-12)
				_, _, wDepth := Project3D(cx, -12.0, cz, camX, camY, camZ)
				entities = append(entities, RenderEntity{
					Depth: wDepth,
					Draw: func(screen *ebiten.Image) {
						l.draw3DBlock(screen, localX, localY, tick, camX, camY, camZ)
					},
				})
			}
		}
	}

	// 4. บรรจุไอเทมบนพื้น
	for i := range l.Items {
		item := &l.Items[i]
		if item.Collected {
			continue
		}
		localItem := item
		cx := (float64(localItem.GridX) - 5.5) * 20.0
		cz := (float64(localItem.GridY) - 5.5) * 20.0
		_, _, depth := Project3D(cx, 0.0, cz, camX, camY, camZ)

		entities = append(entities, RenderEntity{
			Depth: depth + 2.0,
			Draw: func(screen *ebiten.Image) {
				l.draw3DItem(screen, localItem, camX, camY, camZ)
			},
		})
	}

	return entities
}

func (l *Level) draw3DFloor(screen *ebiten.Image, x, y int, camX, camY, camZ float64) {
	x0 := (float64(x) - 6.0) * 20.0
	x1 := (float64(x) - 5.0) * 20.0
	z0 := (float64(y) - 6.0) * 20.0
	z1 := (float64(y) - 5.0) * 20.0
	yVal := 0.0

	p0x, p0y, _ := Project3D(x0, yVal, z0, camX, camY, camZ)
	p1x, p1y, _ := Project3D(x1, yVal, z0, camX, camY, camZ)
	p2x, p2y, _ := Project3D(x1, yVal, z1, camX, camY, camZ)
	p3x, p3y, _ := Project3D(x0, yVal, z1, camX, camY, camZ)

	pts := []pointF{
		{p0x, p0y},
		{p1x, p1y},
		{p2x, p2y},
		{p3x, p3y},
	}

	tile := l.Tiles[y][x]
	var c color.Color

	if tile == TileStairsUp || tile == TileStairsDown {
		// วาดบันไดขั้น 3D perspective
		for step := 0; step < 4; step++ {
			hOffset := float64(step) * 6.0
			stepY := yVal - hOffset
			sp0x, sp0y, _ := Project3D(x0, stepY, z0, camX, camY, camZ)
			sp1x, sp1y, _ := Project3D(x1, stepY, z0, camX, camY, camZ)
			sp2x, sp2y, _ := Project3D(x1, stepY, z1, camX, camY, camZ)
			sp3x, sp3y, _ := Project3D(x0, stepY, z1, camX, camY, camZ)

			spts := []pointF{
				{sp0x, sp0y},
				{sp1x, sp1y},
				{sp2x, sp2y},
				{sp3x, sp3y},
			}

			var stepColor color.Color
			if l.ID <= 3 {
				stepColor = color.RGBA{145 - uint8(step*15), 95 - uint8(step*10), 35, 255}
			} else {
				stepColor = color.RGBA{70 + uint8(step*15), 85 + uint8(step*15), 95, 255}
			}

			drawIsoPolygon(screen, spts, stepColor)
			vector.StrokeLine(screen, sp0x, sp0y, sp1x, sp1y, 0.5, color.RGBA{255, 255, 255, 80}, false)
			vector.StrokeLine(screen, sp1x, sp1y, sp2x, sp2y, 0.5, color.RGBA{255, 255, 255, 80}, false)
			vector.StrokeLine(screen, sp2x, sp2y, sp3x, sp3y, 0.5, color.RGBA{255, 255, 255, 80}, false)
			vector.StrokeLine(screen, sp3x, sp3y, sp0x, sp0y, 0.5, color.RGBA{255, 255, 255, 80}, false)
		}
		return
	}

	if l.ID == 3 {
		if (x+y)%2 == 0 {
			c = color.RGBA{189, 195, 199, 255}
		} else {
			c = color.RGBA{127, 140, 141, 255}
		}
	} else if l.ID == 4 {
		c = color.RGBA{30, 40, 45, 255}
	} else {
		c = color.RGBA{120, 20, 30, 255}
	}

	drawIsoPolygon(screen, pts, c)

	var strokeColor color.Color
	if l.ID == 4 {
		strokeColor = color.RGBA{45, 60, 65, 255}
	} else {
		strokeColor = color.RGBA{145, 95, 35, 100}
	}
	vector.StrokeLine(screen, p0x, p0y, p1x, p1y, 0.5, strokeColor, false)
	vector.StrokeLine(screen, p1x, p1y, p2x, p2y, 0.5, strokeColor, false)
	vector.StrokeLine(screen, p2x, p2y, p3x, p3y, 0.5, strokeColor, false)
	vector.StrokeLine(screen, p3x, p3y, p0x, p0y, 0.5, strokeColor, false)
}

func (l *Level) draw3DGoal(screen *ebiten.Image, x, y int, camX, camY, camZ float64) {
	x0 := (float64(x) - 6.0) * 20.0
	x1 := (float64(x) - 5.0) * 20.0
	z0 := (float64(y) - 6.0) * 20.0
	z1 := (float64(y) - 5.0) * 20.0
	yVal := 0.0

	p0x, p0y, _ := Project3D(x0, yVal, z0, camX, camY, camZ)
	p1x, p1y, _ := Project3D(x1, yVal, z0, camX, camY, camZ)
	p2x, p2y, _ := Project3D(x1, yVal, z1, camX, camY, camZ)
	p3x, p3y, _ := Project3D(x0, yVal, z1, camX, camY, camZ)

	pts := []pointF{
		{p0x, p0y},
		{p1x, p1y},
		{p2x, p2y},
		{p3x, p3y},
	}
	drawIsoPolygon(screen, pts, color.RGBA{241, 196, 15, 80})
	vector.StrokeLine(screen, p0x, p0y, p1x, p1y, 1.5, color.RGBA{197, 160, 89, 255}, false)
	vector.StrokeLine(screen, p1x, p1y, p2x, p2y, 1.5, color.RGBA{197, 160, 89, 255}, false)
	vector.StrokeLine(screen, p2x, p2y, p3x, p3y, 1.5, color.RGBA{197, 160, 89, 255}, false)
	vector.StrokeLine(screen, p3x, p3y, p0x, p0y, 1.5, color.RGBA{197, 160, 89, 255}, false)
}

func (l *Level) draw3DBlock(screen *ebiten.Image, x, y int, tick int, camX, camY, camZ float64) {
	tile := l.Tiles[y][x]
	x0 := (float64(x) - 6.0) * 20.0
	x1 := (float64(x) - 5.0) * 20.0
	z0 := (float64(y) - 6.0) * 20.0
	z1 := (float64(y) - 5.0) * 20.0

	yBottom := 0.0
	yTop := -24.0

	// โปรเจคยอดทั้ง 8 จุดลงหน้าจอ
	_, _, _ = Project3D(x0, yBottom, z0, camX, camY, camZ) // จุดยอดหลังที่บดบังไว้
	p1x, p1y, _ := Project3D(x1, yBottom, z0, camX, camY, camZ)
	p2x, p2y, _ := Project3D(x1, yBottom, z1, camX, camY, camZ)
	p3x, p3y, _ := Project3D(x0, yBottom, z1, camX, camY, camZ)

	p4x, p4y, _ := Project3D(x0, yTop, z0, camX, camY, camZ)
	p5x, p5y, _ := Project3D(x1, yTop, z0, camX, camY, camZ)
	p6x, p6y, _ := Project3D(x1, yTop, z1, camX, camY, camZ)
	p7x, p7y, _ := Project3D(x0, yTop, z1, camX, camY, camZ)

	var topColor, leftColor, rightColor color.Color
	var outlineColor color.Color

	switch tile {
	case TileRedDoor:
		topColor = color.RGBA{220, 80, 80, 255}
		leftColor = color.RGBA{192, 41, 43, 255}
		rightColor = color.RGBA{120, 20, 20, 255}
		outlineColor = color.RGBA{241, 196, 15, 255}

	case TileBlueDoor:
		topColor = color.RGBA{80, 160, 220, 255}
		leftColor = color.RGBA{41, 128, 185, 255}
		rightColor = color.RGBA{20, 70, 120, 255}
		outlineColor = color.RGBA{241, 196, 15, 255}

	case TileSecretBoss:
		topColor = color.RGBA{185, 120, 230, 255}
		leftColor = color.RGBA{155, 89, 182, 255}
		rightColor = color.RGBA{90, 45, 120, 255}
		outlineColor = color.RGBA{255, 255, 255, 200}

	case TileItemBox:
		topColor = color.RGBA{160, 100, 45, 255}
		leftColor = color.RGBA{120, 70, 30, 255}
		rightColor = color.RGBA{80, 45, 20, 255}
		outlineColor = color.RGBA{215, 170, 90, 255}

	default:
		if l.ID <= 3 {
			topColor = color.RGBA{115, 60, 30, 255}
			leftColor = color.RGBA{85, 40, 20, 255}
			rightColor = color.RGBA{50, 20, 10, 255}
			outlineColor = color.RGBA{197, 160, 89, 255}
		} else {
			topColor = color.RGBA{85, 105, 120, 255}
			leftColor = color.RGBA{52, 73, 94, 255}
			rightColor = color.RGBA{35, 50, 65, 255}
			outlineColor = color.RGBA{127, 140, 141, 255}
		}
	}

	// 1. วาดผนังด้านซ้ายเฉียง (SW Face: p3 -> p2 -> p6 -> p7)
	leftPts := []pointF{
		{p3x, p3y},
		{p2x, p2y},
		{p6x, p6y},
		{p7x, p7y},
	}
	drawIsoPolygon(screen, leftPts, leftColor)

	// 2. วาดผนังด้านขวาเฉียง (SE Face: p2 -> p1 -> p5 -> p6)
	rightPts := []pointF{
		{p2x, p2y},
		{p1x, p1y},
		{p5x, p5y},
		{p6x, p6y},
	}
	drawIsoPolygon(screen, rightPts, rightColor)

	// 3. วาดหลังคาแบนด้านบน (Top Face: p4 -> p5 -> p6 -> p7)
	topPts := []pointF{
		{p4x, p4y},
		{p5x, p5y},
		{p6x, p6y},
		{p7x, p7y},
	}
	drawIsoPolygon(screen, topPts, topColor)

	// วาดตัดเส้นขอบ
	vector.StrokeLine(screen, p4x, p4y, p5x, p5y, 1, outlineColor, false)
	vector.StrokeLine(screen, p5x, p5y, p6x, p6y, 1, outlineColor, false)
	vector.StrokeLine(screen, p6x, p6y, p7x, p7y, 1, outlineColor, false)
	vector.StrokeLine(screen, p7x, p7y, p4x, p4y, 1, outlineColor, false)

	if tile == TileRedDoor || tile == TileBlueDoor {
		midX := p6x
		midY := p6y + 10.0
		vector.DrawFilledCircle(screen, midX, midY, 3, color.RGBA{241, 196, 15, 255}, false)
		vector.DrawFilledRect(screen, midX-1, midY, 2, 4, color.RGBA{241, 196, 15, 255}, false)
	}

	if tile == TileSecretBoss {
		midX := p6x
		midY := p6y - 6.0
		pulse := float32(math.Sin(float64(tick)*0.1) * 3)
		vector.DrawFilledCircle(screen, midX, midY, 6+pulse, color.RGBA{155, 89, 182, 200}, false)
		vector.DrawFilledCircle(screen, midX, midY, 3+pulse/2, color.RGBA{255, 255, 255, 255}, false)
	}

	if tile == TileItemBox {
		midX := p6x
		midY := p6y + 6.0
		vector.DrawFilledRect(screen, midX-4, midY-2, 8, 4, color.RGBA{215, 170, 90, 255}, false)
		vector.StrokeRect(screen, midX-4, midY-2, 8, 4, 1.0, color.RGBA{40, 25, 10, 255}, false)
	}
}

func (l *Level) draw3DItem(screen *ebiten.Image, item *Item, camX, camY, camZ float64) {
	cx := (float64(item.GridX) - 5.5) * 20.0
	cz := (float64(item.GridY) - 5.5) * 20.0
	isoX, isoY, depth := Project3D(cx, 0.0, cz, camX, camY, camZ)

	fScale := float32(240.0 / depth)
	if fScale > 2.0 {
		fScale = 2.0
	}

	switch item.Type {
	case ItemRedKey:
		vector.DrawFilledCircle(screen, isoX, isoY-5*fScale, 4*fScale, color.RGBA{231, 76, 60, 255}, false)
		vector.DrawFilledRect(screen, isoX-1*fScale, isoY-1*fScale, 2*fScale, 6*fScale, color.RGBA{231, 76, 60, 255}, false)
		vector.DrawFilledRect(screen, isoX-1*fScale, isoY+2*fScale, 3*fScale, 1*fScale, color.RGBA{231, 76, 60, 255}, false)
		vector.DrawFilledRect(screen, isoX-1*fScale, isoY+4*fScale, 3*fScale, 1*fScale, color.RGBA{231, 76, 60, 255}, false)
	case ItemBlueKey:
		vector.DrawFilledCircle(screen, isoX, isoY-5*fScale, 4*fScale, color.RGBA{52, 152, 219, 255}, false)
		vector.DrawFilledRect(screen, isoX-1*fScale, isoY-1*fScale, 2*fScale, 6*fScale, color.RGBA{52, 152, 219, 255}, false)
		vector.DrawFilledRect(screen, isoX-1*fScale, isoY+2*fScale, 3*fScale, 1*fScale, color.RGBA{52, 152, 219, 255}, false)
		vector.DrawFilledRect(screen, isoX-1*fScale, isoY+4*fScale, 3*fScale, 1*fScale, color.RGBA{52, 152, 219, 255}, false)
	case ItemEnergyChip:
		pts := []pointF{
			{isoX, isoY - 4*fScale},
			{isoX + 6*fScale, isoY},
			{isoX, isoY + 4*fScale},
			{isoX - 6*fScale, isoY},
		}
		drawIsoPolygon(screen, pts, color.RGBA{241, 196, 15, 255})
		vector.DrawFilledCircle(screen, isoX, isoY, 1*fScale, color.RGBA{255, 255, 255, 255}, false)
	}
}
