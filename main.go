package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"log"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/examples/resources/fonts"
	resources "github.com/hajimehoshi/ebiten/v2/examples/resources/images/flappy"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
)

func floorDiv(x, y int) int {
	d := x / y
	if d*y == x || x >= 0 {
		return d
	}
	return d - 1
}

func floorMod(x, y int) int {
	return x - floorDiv(x, y)*y
}

// pipe
// r := image.Rect(pipeTileSrcX, pipeTileSrcY+tileSize, pipeTileSrcX+tileSize*2, pipeTileSrcY+tileSize*2)
// top
// r := image.Rect(pipeTileSrcX, pipeTileSrcY, pipeTileSrcX+pipeWidth, pipeTileSrcY+tileSize)

const (
	screenWidth      = 640
	screenHeight     = 480
	tileSize         = 32
	titleFontSize    = fontSize * 1.5
	fontSize         = 24
	smallFontSize    = fontSize / 2
	pipeWidth        = tileSize * 2
	pipeStartOffsetX = 8
	pipeIntervalX    = 8
	pipeGapY         = 5
)

var (
	gopherImage      *ebiten.Image
	tilesImage       *ebiten.Image
	arcadeFaceSource *text.GoTextFaceSource
)

func init() {
	img, _, err := image.Decode(bytes.NewReader(resources.Gopher_png))
	if err != nil {
		log.Fatal(err)
	}
	gopherImage = ebiten.NewImageFromImage(img)

	img, _, err = image.Decode(bytes.NewReader(resources.Tiles_png))
	if err != nil {
		log.Fatal(err)
	}
	tilesImage = ebiten.NewImageFromImage(img)
}

func init() {
	s, err := text.NewGoTextFaceSource(bytes.NewReader(fonts.PressStart2P_ttf))
	if err != nil {
		log.Fatal(err)
	}
	arcadeFaceSource = s
}

type Game struct {
	mode  int
	score int

	// The gopher's position
	x16  int
	y16  int
	vy16 int

	// Camera
	cameraX int
	cameraY int

	// Pipes
	pipeTileYs []int

	gameoverCount int

	touchIDs   []ebiten.TouchID
	gamepadIDs []ebiten.GamepadID

	audioContext *audio.Context
	jumpPlayer   *audio.Player
	hitPlayer    *audio.Player
}

func (g *Game) init() {
	g.x16 = 0
	g.y16 = 100 * 16
	g.cameraX = -240
	g.cameraY = 0
}

func (g *Game) Update() error {
	g.cameraX += 2
	g.x16 += 32

	// Gravity
	g.vy16 += 4
	if g.vy16 > 96 {
		g.vy16 = 96
	}

	// clamp to ground
	clamp := screenHeight*16 - 1600
	if g.y16+g.vy16 > clamp {
		g.y16 = clamp
	} else {
		g.y16 += g.vy16
	}

	// jump
	if g.isKeyJustPressed() && g.y16 == clamp {
		g.vy16 = -150
	}

	return nil
}

func (g *Game) DrawTiles(screen *ebiten.Image) {
	const (
		nx           = screenWidth / tileSize
		ny           = screenHeight / tileSize
		pipeTileSrcX = 128
		pipeTileSrcY = 192
	)

	op := &ebiten.DrawImageOptions{}

	for i := -2; i < nx+1; i++ {
		// ground
		op.GeoM.Reset()
		op.GeoM.Translate(float64(i*tileSize-floorMod(g.cameraX, tileSize)),
			float64((ny-1)*tileSize-floorMod(g.cameraY, tileSize)))
		screen.DrawImage(tilesImage.SubImage(image.Rect(0, 0, tileSize, tileSize)).(*ebiten.Image), op)
	}

	// pipes
	var r image.Rectangle
	op.GeoM.Reset()
	op.GeoM.Translate(
		float64(-g.cameraX+1000),
		float64((ny-2)*tileSize-floorMod(g.cameraY, tileSize)),
	)
	r = image.Rect(pipeTileSrcX, pipeTileSrcY+tileSize, pipeTileSrcX+tileSize*2, pipeTileSrcY+tileSize*2)
	screen.DrawImage(tilesImage.SubImage(r).(*ebiten.Image), op)
	op.GeoM.Translate(0, -float64(tileSize))
	r = image.Rect(pipeTileSrcX, pipeTileSrcY, pipeTileSrcX+pipeWidth, pipeTileSrcY+tileSize)
	screen.DrawImage(tilesImage.SubImage(r).(*ebiten.Image), op)
}

func (g *Game) drawGopher(screen *ebiten.Image) {
	op := &ebiten.DrawImageOptions{}
	w, h := gopherImage.Bounds().Dx(), gopherImage.Bounds().Dy()
	op.GeoM.Translate(-float64(w)/2.0, -float64(h)/2.0)
	op.GeoM.Rotate(float64(g.vy16) / 96.0 * math.Pi / 6)
	op.GeoM.Translate(float64(w)/2.0, float64(h)/2.0)
	op.GeoM.Translate(float64(g.x16/16.0)-float64(g.cameraX), float64(g.y16/16.0)-float64(g.cameraY))
	op.Filter = ebiten.FilterLinear
	screen.DrawImage(gopherImage, op)
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.RGBA{0x00, 0xbc, 0xff, 0xff})
	ebitenutil.DebugPrint(screen, fmt.Sprintf("cameraX: %v, gopher: %v, %v", g.cameraX*16, g.x16, g.y16))

	op := &text.DrawOptions{}

	g.DrawTiles(screen)
	g.drawGopher(screen)

	// draw score
	op = &text.DrawOptions{}
	op.GeoM.Translate(screenWidth, 0)
	op.ColorScale.ScaleWithColor(color.White)
	op.LineSpacing = fontSize
	op.PrimaryAlign = text.AlignEnd
	text.Draw(screen, fmt.Sprintf("%04d", g.score), &text.GoTextFace{
		Source: arcadeFaceSource,
		Size:   fontSize,
	}, op)
}

func (g *Game) isKeyJustPressed() bool {
	if inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		return true
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return true
	}
	g.touchIDs = inpututil.AppendJustPressedTouchIDs(g.touchIDs[:0])
	if len(g.touchIDs) > 0 {
		return true
	}
	g.gamepadIDs = ebiten.AppendGamepadIDs(g.gamepadIDs[:0])
	for _, g := range g.gamepadIDs {
		if ebiten.IsStandardGamepadLayoutAvailable(g) {
			if inpututil.IsStandardGamepadButtonJustPressed(g, ebiten.StandardGamepadButtonRightBottom) {
				return true
			}
			if inpututil.IsStandardGamepadButtonJustPressed(g, ebiten.StandardGamepadButtonRightRight) {
				return true
			}
		} else {
			// The button 0/1 might not be A/B buttons.
			if inpututil.IsGamepadButtonJustPressed(g, ebiten.GamepadButton0) {
				return true
			}
			if inpututil.IsGamepadButtonJustPressed(g, ebiten.GamepadButton1) {
				return true
			}
		}
	}
	return false
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return screenWidth, screenHeight
}

func main() {
	ebiten.SetWindowSize(screenWidth, screenHeight)
	ebiten.SetWindowTitle("Flappy")

	g := &Game{}
	g.init()
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
