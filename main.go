package main

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"io/fs"
	"log"
	"math"
	"path"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"crg.eti.br/go/config"
	_ "crg.eti.br/go/config/ini"
)

type kurama struct {
	waiting bool
	x       float64
	y           float64
	distance    int
	count       int
	min         int
	max         int
	state       int
	sprite      string
	lastSprite  string
	img         *ebiten.Image

	cfg           *Config
	sprites       map[string]*ebiten.Image
	sounds        map[string][]byte
	audioContext  *audio.Context
	currentPlayer *audio.Player
}

type Config struct {
	Speed            float64 `cfg:"speed" cfgDefault:"10.0" cfgHelper:"The speed of the fox."`
	Scale            float64 `cfg:"scale" cfgDefault:"1.0" cfgHelper:"The scale of the fox."`
	Quiet            bool    `cfg:"quiet" cfgDefault:"false" cfgHelper:"Disable sound."`
	MousePassthrough bool    `cfg:"mousepassthrough" cfgDefault:"false" cfgHelper:"Enable mouse passthrough."`
}

const (
	width       = 64
	height      = 64
	idleRadius  = 80
	sampleRate  = 44100
	soundVolume = 0.3
)

var (
	//go:embed assets/*
	f embed.FS
)

func (m *kurama) Layout(outsideWidth, outsideHeight int) (int, int) {
	return width, height
}

func (m *kurama) playSound(soundName string) {
	if m.cfg.Quiet {
		return
	}
	sound, ok := m.sounds[soundName]
	if !ok {
		return
	}
	if m.currentPlayer != nil {
		_ = m.currentPlayer.Close()
		m.currentPlayer = nil
	}
	m.currentPlayer = m.audioContext.NewPlayerFromBytes(sound)
	m.currentPlayer.SetVolume(soundVolume)
	m.currentPlayer.Play()
}

func (m *kurama) Update() error {
	m.count++
	if m.state == 10 && m.count == m.min {
		m.playSound("idle3")
	}

	// Global cursor position so the fox can follow across monitors and Spaces.
	cursorX, cursorY := globalCursorPosition()

	// Fox center in screen coordinates. Use the actual window size so this
	// stays correct when KURAMA_SCALE != 1.0.
	windowWidth, windowHeight := ebiten.WindowSize()
	halfWidth := float64(windowWidth) / 2
	halfHeight := float64(windowHeight) / 2

	dx := cursorX - (m.x + halfWidth)
	dy := cursorY - (m.y + halfHeight)
	distanceToCursor := math.Hypot(dx, dy)
	m.distance = int(distanceToCursor)

	if distanceToCursor < float64(idleRadius) || m.waiting {
		m.stayIdle()
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			m.waiting = !m.waiting
		}
		ebiten.SetWindowPosition(int(math.Round(m.x)), int(math.Round(m.y)))
		return nil
	}

	if m.state >= 16 {
		m.playSound("awake")
	}
	m.catchCursor(dx, dy, distanceToCursor)
	ebiten.SetWindowPosition(int(math.Round(m.x)), int(math.Round(m.y)))
	return nil
}

func (m *kurama) stayIdle() {
	// idle state
	switch m.state {
	case 0, 1, 2, 3:
		if m.state == 0 {
			m.state = 1
		}
		m.sprite = "awake"

	case 4, 5, 6:
		m.sprite = "scratch"

	case 7, 8, 9:
		m.sprite = "wash"

	case 10, 11, 12:
		m.min = 64
		m.max = 128
		m.sprite = "yawn"

	case 13, 14, 15:
		// brief pause between yawn and sleep
		m.min = 24
		m.max = 48
		m.sprite = "awake"

	default:
		// slow breathing sleep animation
		m.min = 80
		m.max = 160
		m.sprite = "sleep"
	}
}

func (m *kurama) catchCursor(dx, dy, distanceToCursor float64) {
	m.state = 0
	m.min = 8
	m.max = 16

	if distanceToCursor <= 0 {
		return
	}

	// Taper speed as the fox approaches the cursor. Full configured speed
	// when the cursor is >= 2 * idleRadius away, and smoothly down to zero
	// as it nears the idle-radius boundary. This eliminates the overshoot
	// that caused the fox to oscillate around the cursor.
	taperStart := float64(idleRadius)
	taperEnd := 2.0 * float64(idleRadius)
	taper := (distanceToCursor - taperStart) / (taperEnd - taperStart)
	if taper < 0 {
		taper = 0
	}
	if taper > 1 {
		taper = 1
	}
	speed := m.cfg.Speed * taper

	// Move along the exact cursor vector instead of snapping movement to
	// one of 8 cardinal directions. Prevents zig-zag when approaching at
	// off-axis angles.
	m.x += (dx / distanceToCursor) * speed
	m.y += (dy / distanceToCursor) * speed

	// Sprite selection is still quantised to 8 directions since we only
	// have 8 directional sprite sets. The chosen sprite tracks the real
	// movement vector, which is visually continuous.
	angleRadians := math.Atan2(dy, dx)
	angleDegrees := math.Mod((angleRadians/math.Pi*180)+360, 360)
	switch {
	case angleDegrees <= 292.5 && angleDegrees > 247.5:
		m.sprite = "up"
	case angleDegrees <= 337.5 && angleDegrees > 292.5:
		m.sprite = "upright"
	case angleDegrees <= 22.5 || angleDegrees > 337.5:
		m.sprite = "right"
	case angleDegrees <= 67.5 && angleDegrees > 22.5:
		m.sprite = "downright"
	case angleDegrees <= 112.5 && angleDegrees > 67.5:
		m.sprite = "down"
	case angleDegrees <= 157.5 && angleDegrees > 112.5:
		m.sprite = "downleft"
	case angleDegrees <= 202.5 && angleDegrees > 157.5:
		m.sprite = "left"
	case angleDegrees <= 247.5 && angleDegrees > 202.5:
		m.sprite = "upleft"
	}
}

func (m *kurama) Draw(screen *ebiten.Image) {
	var sprite string
	switch {
	case m.sprite == "awake":
		sprite = m.sprite
	case m.count < m.min:
		sprite = m.sprite + "1"
	default:
		sprite = m.sprite + "2"
	}

	m.img = m.sprites[sprite]

	if m.count > m.max {
		m.count = 0

		if m.state > 0 {
			m.state++
			switch m.state {
			case 16:
				m.playSound("sleep")
			}
		}
	}

	screen.Clear()
	screen.DrawImage(m.img, nil)
}

func loadAssets(assetsFS fs.FS, sampleRate int) (map[string]*ebiten.Image, map[string][]byte, error) {
	sprites := make(map[string]*ebiten.Image)
	sounds := make(map[string][]byte)

	entries, err := fs.ReadDir(assetsFS, "assets")
	if err != nil {
		return nil, nil, fmt.Errorf("read assets directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		assetPath := path.Join("assets", entry.Name())
		data, err := fs.ReadFile(assetsFS, assetPath)
		if err != nil {
			return nil, nil, fmt.Errorf("read %q: %w", assetPath, err)
		}

		name := strings.TrimSuffix(entry.Name(), path.Ext(entry.Name()))
		switch path.Ext(entry.Name()) {
		case ".png":
			img, _, err := image.Decode(bytes.NewReader(data))
			if err != nil {
				return nil, nil, fmt.Errorf("decode sprite %q: %w", entry.Name(), err)
			}
			sprites[name] = ebiten.NewImageFromImage(img)

		case ".wav":
			stream, err := wav.DecodeWithSampleRate(sampleRate, bytes.NewReader(data))
			if err != nil {
				return nil, nil, fmt.Errorf("decode sound %q: %w", entry.Name(), err)
			}
			soundData, err := io.ReadAll(stream)
			if err != nil {
				return nil, nil, fmt.Errorf("read sound %q: %w", entry.Name(), err)
			}
			sounds[name] = soundData
		}
	}

	return sprites, sounds, nil
}

func main() {
	cfg := &Config{}

	config.PrefixEnv = "KURAMA"
	config.File = "kurama.ini"
	if err := config.Parse(cfg); err != nil {
		log.Fatal(err)
	}

	sprites, sounds, err := loadAssets(f, sampleRate)
	if err != nil {
		log.Fatal(err)
	}

	audioContext := audio.NewContext(sampleRate)

	// Workaround: for some reason playing the first sound can incur significant delay.
	// So let's do this at the start.
	audioContext.NewPlayerFromBytes([]byte{}).Play()

	monitorWidth, monitorHeight := ebiten.Monitor().Size()

	n := &kurama{
		x:            float64(monitorWidth / 2),
		y:            float64(monitorHeight / 2),
		min:          8,
		max:          16,
		cfg:          cfg,
		sprites:      sprites,
		sounds:       sounds,
		audioContext: audioContext,
	}

	ebiten.SetRunnableOnUnfocused(true)
	ebiten.SetScreenClearedEveryFrame(false)
	ebiten.SetTPS(50)
	ebiten.SetVsyncEnabled(true)
	ebiten.SetWindowDecorated(false)
	ebiten.SetWindowFloating(true)
	ebiten.SetWindowMousePassthrough(cfg.MousePassthrough)
	ebiten.SetWindowSize(int(float64(width)*cfg.Scale), int(float64(height)*cfg.Scale))
	ebiten.SetWindowTitle("Kurama")

	// Trigger once after the window is ready. The C code installs an
	// NSTimer on the main run loop that re-applies the flags every 2 s.
	go func() {
		time.Sleep(300 * time.Millisecond)
		setWindowOnAllSpaces()
	}()

	err = ebiten.RunGameWithOptions(n, &ebiten.RunGameOptions{
		InitUnfocused:     true,
		ScreenTransparent: true,
		SkipTaskbar:       true,
		X11ClassName:      "Kurama",
		X11InstanceName:   "Kurama",
	})
	if err != nil {
		log.Fatal(err)
	}
}
