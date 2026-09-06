# Security & Sensitive Data

## 🔒 Protected Files

The following files contain sensitive data and are **automatically excluded** from Git:

### Database Files
- `data/` - Contains SQLite database with account credentials
- `*.db`, `*.sqlite`, `*.sqlite3` - All database files
- **Never commit these** - they contain sessionKeys and account information

### Configuration Files
- `config.yaml` - Contains admin passwords and system settings
- `*.env` - Environment variables
- Use `config.example.yaml` as a template

### Keys & Credentials
- `sessionKey.txt` - Session keys for accounts
- `*.key`, `*.pem` - Private keys and certificates
- `*.p12`, `*.pfx` - Certificate bundles

### Build Artifacts
- `*.exe` - Compiled executables
- `claude2api` - Built binary (Linux/Mac)

### Logs
- `logs/` - Runtime logs may contain sensitive information
- `*.log` - All log files

## 🛡️ Best Practices

### 1. **Never commit real credentials**
```yaml
# ❌ BAD - config.yaml (real credentials)
admin_password: MyRealPassword123

# ✅ GOOD - config.example.yaml (example only)
admin_password: your_secure_password_here
```

### 2. **Before committing, check for sensitive data**
```bash
# Check what will be committed
git status

# Check if a file is ignored
git check-ignore -v data/app.db

# View changes before commit
git diff
```

### 3. **If you accidentally committed sensitive data**
```bash
# Remove file from Git history (but keep local file)
git rm --cached data/app.db

# Commit the removal
git commit -m "Remove sensitive database from Git"

# For files already in Git history, use git-filter-repo or BFG Repo-Cleaner
```

### 4. **Sharing configuration safely**
Always use the example config:
```bash
# Copy example to create your config
cp config.example.yaml config.yaml

# Edit with your real values
notepad config.yaml

# Only commit the example
git add config.example.yaml
```

## 📋 Pre-Commit Checklist

Before running `git commit`, verify:

- [ ] No `*.db` or `*.sqlite` files in staging
- [ ] No `config.yaml` with real passwords
- [ ] No `sessionKey.txt` or similar credential files
- [ ] No `logs/` directory
- [ ] No compiled binaries (`*.exe`)
- [ ] `.env` files are not staged

## 🔍 Check Current Git Status

Run this to see what's tracked:
```bash
# See all changes
git status

# See only tracked files that shouldn't be
git ls-files | grep -E "(\.db|config\.yaml|sessionKey|\.log)"
```

## ⚠️ Important Notes

1. **Database Security**: The `data/app.db` file contains:
   - Account emails
   - SessionKeys (authentication cookies)
   - API keys
   - Session limits and usage data

2. **Config Security**: The `config.yaml` file contains:
   - Admin dashboard password
   - Proxy settings (may include credentials)
   - System configuration

3. **Log Security**: Log files may contain:
   - API requests and responses
   - Account emails
   - Error messages with sensitive context

## 🚨 If Credentials Are Leaked

If you accidentally push sensitive data to GitHub:

1. **Immediately rotate all credentials**:
   - Change admin password
   - Refresh all sessionKeys
   - Regenerate API keys

2. **Remove from Git history**:
   ```bash
   # Install BFG Repo-Cleaner
   # Then remove sensitive files
   bfg --delete-files app.db
   bfg --delete-files config.yaml
   
   # Or use git-filter-repo
   git filter-repo --path data/app.db --invert-paths
   ```

3. **Force push** (if repository is not shared):
   ```bash
   git push --force
   ```

4. **If repository is public**: Consider it compromised, rotate everything.

## 📞 Questions?

If you're unsure whether a file contains sensitive data, **don't commit it**. 
Ask first or check the `.gitignore` file.
