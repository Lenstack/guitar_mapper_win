//go:build windows
// +build windows

package main

import (
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// --- Windows API ---
var (
	user32        = windows.NewLazySystemDLL("user32.dll")
	procSendInput = user32.NewProc("SendInput")
)

// INPUT structs para SendInput
type KEYBDINPUT struct {
	WVk         uint16
	WScan       uint16
	DwFlags     uint32
	Time        uint32
	DwExtraInfo uintptr
}

type INPUT struct {
	Type uint32
	Ki   KEYBDINPUT
}

const (
	INPUT_KEYBOARD  = 1
	KEYEVENTF_KEYUP = 0x0002
)

// --- Mapeo de botones Guitar Hero ---
var buttonMap = map[uint16]uint16{
	0: 0x31, // 1 => '1'
	1: 0x32, // 2 => '2'
	2: 0x33, // 3 => '3'
	3: 0x34, // 4 => '4'
	4: 0x35, // 5 => '5'
	5: 0x5A, // Z => Strum Up
	6: 0x58, // X => Strum Down
	7: 0x20, // Space => Star Power
	8: 0x08, // Backspace
}

var whammyActive bool

// Estado previo de teclas
var prevState = make(map[uint16]bool)

// --- SendInput con batching ---
func SendBatch(events []INPUT) {
	if len(events) == 0 {
		return
	}
	procSendInput.Call(uintptr(len(events)), uintptr(unsafe.Pointer(&events[0])), unsafe.Sizeof(INPUT{}))
}

// --- XInput ---
var (
	xinput             = windows.NewLazySystemDLL("xinput1_4.dll")
	procXInputGetState = xinput.NewProc("XInputGetState")
)

type XINPUT_STATE struct {
	DwPacketNumber uint32
	Gamepad        XINPUT_GAMEPAD
}

type XINPUT_GAMEPAD struct {
	WButtons      uint16
	BLeftTrigger  byte
	BRightTrigger byte
	SThumbLX      int16
	SThumbLY      int16
	SThumbRX      int16
	SThumbRY      int16
}

func GetState(controllerID uint32) (*XINPUT_STATE, error) {
	var state XINPUT_STATE
	ret, _, _ := procXInputGetState.Call(
		uintptr(controllerID),
		uintptr(unsafe.Pointer(&state)),
	)
	if ret != 0 {
		return nil, fmt.Errorf("controlador no conectado")
	}
	return &state, nil
}

func main() {
	fmt.Println("🎸 Leyendo eventos de Guitar Hero en Windows (batching)...")

	ticker := time.NewTicker(1 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		state, err := GetState(0)
		if err != nil {
			continue
		}

		var batch []INPUT

		// --- Manejo de botones ---
		for bit, vk := range buttonMap {
			pressed := (state.Gamepad.WButtons & (1 << bit)) != 0
			prev, exists := prevState[vk]
			if !exists || prev != pressed {
				prevState[vk] = pressed
				flags := uint32(0)
				if !pressed {
					flags = KEYEVENTF_KEYUP
				}
				batch = append(batch, INPUT{
					Type: INPUT_KEYBOARD,
					Ki: KEYBDINPUT{
						WVk:     vk,
						WScan:   0,
						DwFlags: flags,
						Time:    0,
					},
				})
			}
		}

		// --- Whammy optimizado ---
		whammy := state.Gamepad.SThumbRY
		if whammy > 6000 && !whammyActive {
			whammyActive = true
			batch = append(batch, INPUT{
				Type: INPUT_KEYBOARD,
				Ki: KEYBDINPUT{
					WVk:     0x11, // Ctrl
					DwFlags: 0,
				},
			})
		} else if whammy < 4000 && whammyActive {
			whammyActive = false
			batch = append(batch, INPUT{
				Type: INPUT_KEYBOARD,
				Ki: KEYBDINPUT{
					WVk:     0x11, // Ctrl
					DwFlags: KEYEVENTF_KEYUP,
				},
			})
		}

		// Enviar todos los eventos en un solo SendInput
		SendBatch(batch)
	}
}
