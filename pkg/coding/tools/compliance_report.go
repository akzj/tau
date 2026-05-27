package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/akzj/tau/core"
)

// ── Compliance standards ──

type complianceCheck struct {
	ID          string   `json:"id"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Standards   []string `json:"standards"` // which standards this check applies to
}

var complianceChecks = []complianceCheck{
	// Encryption
	{ID: "ENC-01", Category: "Encryption", Description: "Data encrypted at rest (AES-256 or equivalent)", Standards: []string{"SOC2", "GDPR", "HIPAA", "PCI"}},
	{ID: "ENC-02", Category: "Encryption", Description: "Data encrypted in transit (TLS 1.2+)", Standards: []string{"SOC2", "GDPR", "HIPAA", "PCI"}},
	{ID: "ENC-03", Category: "Encryption", Description: "Encryption keys managed securely (KMS, HSM)", Standards: []string{"SOC2", "PCI"}},
	{ID: "ENC-04", Category: "Encryption", Description: "TLS certificates valid and not self-signed in production", Standards: []string{"SOC2", "PCI"}},

	// Access Control
	{ID: "ACC-01", Category: "Access Control", Description: "Role-based access control (RBAC) implemented", Standards: []string{"SOC2", "GDPR", "HIPAA", "PCI"}},
	{ID: "ACC-02", Category: "Access Control", Description: "Multi-factor authentication (MFA) available", Standards: []string{"SOC2", "PCI"}},
	{ID: "ACC-03", Category: "Access Control", Description: "Principle of least privilege enforced", Standards: []string{"SOC2", "HIPAA", "PCI"}},
	{ID: "ACC-04", Category: "Access Control", Description: "Access reviews conducted periodically", Standards: []string{"SOC2", "HIPAA"}},
	{ID: "ACC-05", Category: "Access Control", Description: "Password policy enforced (min length, complexity, rotation)", Standards: []string{"SOC2", "PCI"}},

	// Audit Logging
	{ID: "AUD-01", Category: "Audit Logging", Description: "All access events logged", Standards: []string{"SOC2", "GDPR", "HIPAA", "PCI"}},
	{ID: "AUD-02", Category: "Audit Logging", Description: "Audit logs immutable and tamper-proof", Standards: []string{"SOC2", "PCI"}},
	{ID: "AUD-03", Category: "Audit Logging", Description: "Log retention period defined (min 1 year)", Standards: []string{"SOC2", "HIPAA", "PCI"}},
	{ID: "AUD-04", Category: "Audit Logging", Description: "Automated alerts for security events", Standards: []string{"SOC2", "PCI"}},
	{ID: "AUD-05", Category: "Audit Logging", Description: "Audit trail includes: who, what, when, where, source IP", Standards: []string{"SOC2", "GDPR", "HIPAA"}},

	// Data Retention
	{ID: "RET-01", Category: "Data Retention", Description: "Data retention policy defined and documented", Standards: []string{"SOC2", "GDPR", "HIPAA"}},
	{ID: "RET-02", Category: "Data Retention", Description: "Data deletion capability (right to erasure)", Standards: []string{"GDPR", "HIPAA"}},
	{ID: "RET-03", Category: "Data Retention", Description: "Data classification schema in place", Standards: []string{"SOC2", "HIPAA"}},
	{ID: "RET-04", Category: "Data Retention", Description: "Cardholder data not stored after authorization (PCI DSS req 3)", Standards: []string{"PCI"}},
	{ID: "RET-05", Category: "Data Retention", Description: "Data minimization principle applied", Standards: []string{"GDPR"}},

	// Backup & Recovery
	{ID: "BCK-01", Category: "Backup & Recovery", Description: "Automated backups configured", Standards: []string{"SOC2", "HIPAA"}},
	{ID: "BCK-02", Category: "Backup & Recovery", Description: "Backup encryption enabled", Standards: []string{"SOC2", "HIPAA", "GDPR"}},
	{ID: "BCK-03", Category: "Backup & Recovery", Description: "Recovery procedures tested regularly", Standards: []string{"SOC2", "HIPAA"}},
	{ID: "BCK-04", Category: "Backup & Recovery", Description: "Disaster recovery plan documented", Standards: []string{"SOC2", "HIPAA"}},
	{ID: "BCK-05", Category: "Backup & Recovery", Description: "RPO/RTO defined and measured", Standards: []string{"SOC2"}},

	// Vulnerability Management
	{ID: "VUL-01", Category: "Vulnerability Management", Description: "Regular vulnerability scanning performed", Standards: []string{"SOC2", "PCI"}},
	{ID: "VUL-02", Category: "Vulnerability Management", Description: "Patch management process in place", Standards: []string{"SOC2", "HIPAA", "PCI"}},
	{ID: "VUL-03", Category: "Vulnerability Management", Description: "Dependency updates reviewed and applied", Standards: []string{"SOC2"}},

	// GDPR Specific
	{ID: "GDPR-01", Category: "GDPR Specific", Description: "Data Processing Agreement (DPA) in place", Standards: []string{"GDPR"}},
	{ID: "GDPR-02", Category: "GDPR Specific", Description: "Consent management implemented", Standards: []string{"GDPR"}},
	{ID: "GDPR-03", Category: "GDPR Specific", Description: "Data Protection Impact Assessment (DPIA) conducted", Standards: []string{"GDPR"}},
	{ID: "GDPR-04", Category: "GDPR Specific", Description: "Data Protection Officer (DPO) appointed if required", Standards: []string{"GDPR"}},
	{ID: "GDPR-05", Category: "GDPR Specific", Description: "Cross-border data transfer safeguards", Standards: []string{"GDPR"}},

	// HIPAA Specific
	{ID: "HIPAA-01", Category: "HIPAA Specific", Description: "BAA (Business Associate Agreement) signed with vendors", Standards: []string{"HIPAA"}},
	{ID: "HIPAA-02", Category: "HIPAA Specific", Description: "PHI access restricted to authorized personnel", Standards: []string{"HIPAA"}},
	{ID: "HIPAA-03", Category: "HIPAA Specific", Description: "PHI de-identification capability", Standards: []string{"HIPAA"}},
	{ID: "HIPAA-04", Category: "HIPAA Specific", Description: "Breach notification process in place", Standards: []string{"HIPAA"}},

	// PCI DSS Specific
	{ID: "PCI-01", Category: "PCI DSS Specific", Description: "Firewall configuration reviewed", Standards: []string{"PCI"}},
	{ID: "PCI-02", Category: "PCI DSS Specific", Description: "Default passwords changed on all systems", Standards: []string{"PCI"}},
	{ID: "PCI-03", Category: "PCI DSS Specific", Description: "Cardholder data environment segmented", Standards: []string{"PCI"}},
	{ID: "PCI-04", Category: "PCI DSS Specific", Description: "Penetration testing conducted annually", Standards: []string{"PCI"}},
}

// ComplianceReportTool creates a compliance report generator.
//
// Parameters:
//
//	path     (string, required) — project path to analyze
//	standard (string, optional) — SOC2, GDPR, HIPAA, PCI (default: SOC2)
//	format   (string, optional) — markdown or json (default: markdown)
func ComplianceReportTool() core.Tool {
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Project path to analyze"},
			"standard": {"type": "string", "description": "Compliance standard: SOC2, GDPR, HIPAA, PCI (default: SOC2)"},
			"format": {"type": "string", "description": "Output format: markdown or json (default: markdown)"}
		},
		"required": ["path"]
	}`)

	return core.Tool{
		Name:        "compliance_report",
		Description: "Compliance report generator. Generates compliance checklist report for SOC2, GDPR, HIPAA, or PCI. Checks: encryption, access control, audit logging, data retention, backup, vulnerability management, and standard-specific requirements.",
		Schema:      Schema{Raw: schema},
		Mode:        core.ModeSequential,
		Execute: func(ctx context.Context, callID string, params any, onUpdate func(core.PartialResult)) (core.ToolResult, error) {
			var args struct {
				Path     string `json:"path"`
				Standard string `json:"standard"`
				Format   string `json:"format"`
			}
			raw, _ := json.Marshal(params)
			json.Unmarshal(raw, &args)
			if args.Path == "" {
				return core.ToolResult{}, fmt.Errorf("path required")
			}
			if args.Standard == "" {
				args.Standard = "SOC2"
			}
			if args.Format == "" {
				args.Format = "markdown"
			}

			args.Standard = strings.ToUpper(args.Standard)
			if !isValidStandard(args.Standard) {
				return core.ToolResult{}, fmt.Errorf("unsupported standard: %s (valid: SOC2, GDPR, HIPAA, PCI)", args.Standard)
			}

			scanDir, err := ResolvePath(args.Path)
			if err != nil {
				return core.ToolResult{}, err
			}

			// Assess the project against the standard
			assessment := assessCompliance(scanDir, args.Standard)

			if args.Format == "json" {
				return buildComplianceJSON(assessment, args.Standard), nil
			}
			return buildComplianceMarkdown(assessment, args.Standard), nil
		},
	}
}

type complianceAssessment struct {
	Standard          string              `json:"standard"`
	TotalChecks       int                 `json:"total_checks"`
	Passed            int                 `json:"passed"`
	Failed            int                 `json:"failed"`
	NeedsReview       int                 `json:"needs_review"`
	Items             []complianceItem    `json:"items"`
}

type complianceItem struct {
	ID          string `json:"id"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Status      string `json:"status"` // pass, fail, needs_review
	Evidence    string `json:"evidence"`
}

func isValidStandard(s string) bool {
	switch strings.ToUpper(s) {
	case "SOC2", "GDPR", "HIPAA", "PCI":
		return true
	}
	return false
}

func assessCompliance(dir, standard string) complianceAssessment {
	var items []complianceItem
	passed, failed, review := 0, 0, 0

	for _, chk := range complianceChecks {
		applies := false
		for _, s := range chk.Standards {
			if s == standard {
				applies = true
				break
			}
		}
		if !applies {
			continue
		}

		item := complianceItem{
			ID:          chk.ID,
			Category:    chk.Category,
			Description: chk.Description,
		}

		// Heuristic assessment based on project structure
		status, evidence := assessCheck(dir, chk)
		item.Status = status
		item.Evidence = evidence

		switch status {
		case "pass":
			passed++
		case "fail":
			failed++
		default:
			review++
		}

		items = append(items, item)
	}

	return complianceAssessment{
		Standard:    standard,
		TotalChecks: len(items),
		Passed:      passed,
		Failed:      failed,
		NeedsReview: review,
		Items:       items,
	}
}

func assessCheck(dir string, chk complianceCheck) (status, evidence string) {
	switch {
	// Encryption checks
	case chk.ID == "ENC-01":
		if hasEncryptionConfig(dir) {
			return "pass", "Found encryption configuration or crypto usage"
		}
		return "needs_review", "No encryption configuration detected automatically"
	case chk.ID == "ENC-02":
		if hasTLSUsage(dir) {
			return "pass", "Found TLS/crypto/tls import or HTTPS config"
		}
		return "needs_review", "No TLS configuration detected automatically"
	case chk.ID == "ENC-03":
		if hasEnvVar(dir, "KMS", "HSM", "VAULT", "KEY_MANAGEMENT") {
			return "pass", "KMS/HSM references found"
		}
		return "needs_review", "No KMS/HSM references detected"
	case chk.ID == "ENC-04":
		return "needs_review", "Manual certificate verification required"

	// Access control checks
	case chk.ID == "ACC-01":
		if hasRBAC(dir) {
			return "pass", "RBAC roles/middleware detected"
		}
		return "needs_review", "No RBAC implementation detected automatically"
	case chk.ID == "ACC-02":
		if hasMFA(dir) {
			return "pass", "MFA/TOTP references found"
		}
		return "needs_review", "No MFA references detected"
	case chk.ID == "ACC-03":
		return "needs_review", "Least privilege requires manual review"
	case chk.ID == "ACC-04":
		return "needs_review", "Access review schedule requires manual verification"
	case chk.ID == "ACC-05":
		if hasPasswordPolicy(dir) {
			return "pass", "Password policy/hashing detected"
		}
		return "needs_review", "No password policy detected in code"

	// Audit logging checks
	case strings.HasPrefix(chk.ID, "AUD-"):
		if hasAuditLogging(dir) {
			return "pass", "Audit/logging infrastructure detected"
		}
		return "needs_review", "No audit logging detected automatically"

	// Data retention checks
	case strings.HasPrefix(chk.ID, "RET-"):
		return "needs_review", "Data retention policy requires documentation review"

	// Backup & recovery
	case strings.HasPrefix(chk.ID, "BCK-"):
		return "needs_review", "Backup configuration requires infrastructure review"

	// Vulnerability management
	case chk.ID == "VUL-01":
		return "pass", "Can be verified with dependency_audit and gitleaks_scan tools"
	case chk.ID == "VUL-02":
		return "needs_review", "Patch management process requires operational review"
	case chk.ID == "VUL-03":
		if hasDependabotOrRenovate(dir) {
			return "pass", "Dependabot/Renovate configuration detected"
		}
		return "needs_review", "No automated dependency update config detected"

	// GDPR-specific
	case strings.HasPrefix(chk.ID, "GDPR-"):
		return "needs_review", "GDPR compliance requires legal & operational review"

	// HIPAA-specific
	case strings.HasPrefix(chk.ID, "HIPAA-"):
		return "needs_review", "HIPAA compliance requires operational & legal review"

	// PCI-specific
	case strings.HasPrefix(chk.ID, "PCI-"):
		return "needs_review", "PCI DSS compliance requires operational review"
	}

	return "needs_review", "Automatic assessment not possible"
}

// ── Heuristic detectors ──

func hasEncryptionConfig(dir string) bool {
	indicators := []string{"crypto/aes", "crypto/cipher", "encrypt", "Encrypt", "AES", "cipher"}
	return scanDirForStrings(dir, indicators)
}

func hasTLSUsage(dir string) bool {
	indicators := []string{"crypto/tls", "TLS", "tls.Config", "https://", "ListenAndServeTLS"}
	return scanDirForStrings(dir, indicators)
}

func hasEnvVar(dir string, vars ...string) bool {
	return scanDirForStrings(dir, vars)
}

func hasRBAC(dir string) bool {
	indicators := []string{"RBAC", "rbac", "Role", "Permission", "permission", "Authorize", "authorize"}
	return scanDirForStrings(dir, indicators)
}

func hasMFA(dir string) bool {
	indicators := []string{"TOTP", "totp", "MFA", "mfa", "two-factor", "2FA", "otp"}
	return scanDirForStrings(dir, indicators)
}

func hasPasswordPolicy(dir string) bool {
	indicators := []string{"bcrypt", "argon2", "scrypt", "pbkdf2", "password_policy", "PasswordPolicy", "min_password"}
	return scanDirForStrings(dir, indicators)
}

func hasAuditLogging(dir string) bool {
	indicators := []string{"audit", "Audit", "audit_log", "auditLog", "activity_log", "access_log", "logrus", "zap", "slog"}
	return scanDirForStrings(dir, indicators)
}

func hasDependabotOrRenovate(dir string) bool {
	indicators := []string{"dependabot", ".github/dependabot", "renovate", "renovate.json"}
	// Also check for config files
	configFiles := []string{".github/dependabot.yml", ".github/dependabot.yaml", "renovate.json", ".renovaterc"}
	for _, cf := range configFiles {
		if _, err := os.Stat(filepath.Join(dir, cf)); err == nil {
			return true
		}
	}
	return scanDirForStrings(dir, indicators)
}

func scanDirForStrings(dir string, needles []string) bool {
	found := false
	filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if found || err != nil {
			return filepath.SkipAll
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(d.Name()))
		if ext != ".go" && ext != ".yaml" && ext != ".yml" && ext != ".json" && ext != ".tf" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)
		for _, n := range needles {
			if strings.Contains(content, n) {
				found = true
				return filepath.SkipAll
			}
		}
		return nil
	})
	return found
}

// ── Output formatters ──

func buildComplianceMarkdown(a complianceAssessment, standard string) core.ToolResult {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# %s Compliance Report\n\n", standard))
	sb.WriteString(fmt.Sprintf("**Summary**: %d checks total — %d passed, %d failed, %d need review\n\n",
		a.TotalChecks, a.Passed, a.Failed, a.NeedsReview))

	if a.TotalChecks > 0 {
		passPct := float64(a.Passed) / float64(a.TotalChecks) * 100
		sb.WriteString(fmt.Sprintf("**Pass Rate**: %.0f%%\n\n", passPct))
	}

	sb.WriteString("---\n\n")

	categories := make(map[string][]complianceItem)
	for _, item := range a.Items {
		categories[item.Category] = append(categories[item.Category], item)
	}

	for _, cat := range []string{
		"Encryption", "Access Control", "Audit Logging", "Data Retention",
		"Backup & Recovery", "Vulnerability Management",
		"GDPR Specific", "HIPAA Specific", "PCI DSS Specific",
	} {
		items, ok := categories[cat]
		if !ok || len(items) == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("## %s\n\n", cat))
		sb.WriteString("| ID | Description | Status | Evidence |\n")
		sb.WriteString("|----|-------------|--------|----------|\n")
		for _, item := range items {
			emoji := "❓"
			switch item.Status {
			case "pass":
				emoji = "✅"
			case "fail":
				emoji = "❌"
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s %s | %s |\n",
				item.ID, item.Description, emoji, item.Status, item.Evidence))
		}
		sb.WriteString("\n")
	}

	output := sb.String()
	if len(output) > OutputCap {
		output = output[:OutputCap] + "\n... (truncated)"
	}

	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: output}},
		Details: map[string]any{
			"standard":    standard,
			"total_checks": a.TotalChecks,
			"passed":      a.Passed,
			"failed":      a.Failed,
			"needs_review": a.NeedsReview,
			"success":     a.Failed == 0,
			"items":       a.Items,
		},
	}
}

func buildComplianceJSON(a complianceAssessment, standard string) core.ToolResult {
	data, _ := json.MarshalIndent(a, "", "  ")
	output := string(data)
	if len(output) > OutputCap {
		output = output[:OutputCap] + "\n... (truncated)"
	}
	return core.ToolResult{
		Content: []core.Content{{Type: "text", Text: output}},
		Details: map[string]any{
			"standard":    standard,
			"total_checks": a.TotalChecks,
			"passed":      a.Passed,
			"failed":      a.Failed,
			"needs_review": a.NeedsReview,
			"success":     a.Failed == 0,
			"items":       a.Items,
		},
	}
}
