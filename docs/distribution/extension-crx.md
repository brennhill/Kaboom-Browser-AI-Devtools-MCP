---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Chrome Extension CRX Distribution

> **Status:** Enables direct installation of Kaboom extension while waiting for Chrome Web Store approval

## Quick Start

### Prerequisites

Install the `crx` Rust tool (recommended):

```bash
cargo install crx
```

Then build:

```bash
make extension-crx  # Build the signed CRX file
```

The CRX file will be created at `dist/kaboom-extension-v[VERSION].crx` with automatic extension ID computation.

**Note:** The build script tries multiple methods in order:

1. **crx tool** (Rust) - Recommended, most reliable
2. **Chrome native** - If Chrome is installed
3. **Manual CRX building** - Fallback if neither is available

## How It Works

### CRX Format & Signing
- **Format:** CRX3 (Chrome Extension version 3)
- **Algorithm:** RSA-2048 with SHA-256
- **Private Key:** `~/.kaboom/extension-signing-key.pem` (generated separately)
- **Extension ID:** Deterministically derived from the public key's SHA-256 hash (base32)

### Key Management
The extension ID is locked to the private key. This means:
- ✅ Same key = same extension ID across all distributions
- ✅ If you generate a new key, you get a new extension ID
- ⚠️ Lose the key = can't update existing installations

Current signing key location: `~/.kaboom/extension-signing-key.pem`

## Distribution Setup

### Step 1: Generate the CRX
```bash
make extension-crx
```

Output:
```
✨ CRX file created: dist/kaboom-extension-v5.8.2.crx
📦 Extension ID: behrmkvjipzkr7hu6mwmbt5vpdgcdyvk
```

### Step 2: Host on gokaboom.dev

Upload the CRX to: `https://gokaboom.dev/downloads/kaboom-extension-v5.8.2.crx`

Typical directory structure:
```
gokaboom.dev/
  downloads/
    kaboom-extension-v5.8.2.crx
    latest.crx → kaboom-extension-v5.8.2.crx (symlink for auto-update)
    checksums.txt
```

### Step 3: User Installation

Users follow these steps:
1. Download the `.crx` file from your website
2. Open Chrome and navigate to `chrome://extensions/`
3. Enable "Developer mode" (toggle in top-right corner)
4. Drag and drop the `.crx` file onto the page
5. Click "Add extension" to confirm

**Note:** Users will see a "Developer mode extensions" warning. This is normal for side-loaded extensions.

## Web Store vs. CRX Comparison

| Feature | Web Store | CRX Distribution |
|---------|-----------|------------------|
| Installation | Easy (1-click) | Moderate (drag & drop) |
| Updates | Automatic | Manual (user downloads new version) |
| Extension ID | Google manages | You control (from key) |
| Review process | 1-2 weeks | None (you own the code) |
| User warning | None | "Developer mode" warning |

## Release Workflow

### Local Build (Recommended)

```bash
# 1. Ensure extension is compiled
make compile-ts

# 2. Build the CRX
make extension-crx

# 3. Run tests (optional but recommended)
make test-js

# 4. Upload CRX to gokaboom.dev
# 5. Update version in VERSION file for next release
# 6. Commit and push
```

### Automated Build with GitHub Actions

For CI/CD automation, use the Sign Chrome Extension CRX File action:

```yaml
- uses: cardinalby/webext-buildtools-chrome-crx-action@v2
  with:
    zipFilePath: 'build/extension.zip'
    crxFilePath: 'build/extension.crx'
    privateKey: ${{ secrets.CHROME_CRX_PRIVATE_KEY }}
    updateXmlPath: 'build/update.xml'
    updateXmlCodebaseUrl: 'https://gokaboom.dev/downloads/kaboom-extension.crx'
```

**Setup:**

1. Store your private key in GitHub Secrets as `CHROME_CRX_PRIVATE_KEY`
2. Add the action to your release workflow
3. Automatically generates `.crx` and optional `update.xml` for auto-updates

## Handling Multiple Distributions

When you eventually get Web Store approval:

1. **Keep the CRX key** — It identifies users who installed via CRX
2. **Use Web Store key** — Users installing from Web Store get the Web Store ID
3. **Both coexist** — Users have two separate extension instances (different data, settings)

**Recommendation:** Once Web Store approval is confirmed, you can:
- Keep CRX distribution as a fallback for users who can't use the Web Store
- OR switch entirely to Web Store and deprecate CRX (requiring users to uninstall and reinstall)

## Updating After Release

To publish a new version:

```bash
# 1. Update version in VERSION file
# 2. Run build and tests
make compile-ts test-js

# 3. Generate new CRX (automatically updates extension ID through manifest)
make extension-crx

# 4. Upload to gokaboom.dev/downloads/kaboom-extension-v[NEW_VERSION].crx
# 5. Update symlink: latest.crx → kaboom-extension-v[NEW_VERSION].crx
```

Users will need to manually download and install the new version since we're not using auto-updates yet.

## Future: Auto-Update via Update Manifest

For fully automated updates, you can later implement:

```xml
<!-- update_manifest.xml hosted on your domain -->
<?xml version='1.0' encoding='UTF-8'?>
<gupdate xmlns='http://www.google.com/update2/response' protocol='3.0'>
  <app appid='behrmkvjipzkr7hu6mwmbt5vpdgcdyvk'>
    <updatecheck codebase='https://gokaboom.dev/downloads/kaboom-extension-v5.8.2.crx' version='5.8.2' />
  </app>
</gupdate>
```

Add to manifest.json:
```json
{
  "update_url": "https://gokaboom.dev/update_manifest.xml"
}
```

Then users get automatic updates without re-downloading.

## Troubleshooting

### "Failed to read private key"
```bash
# Regenerate key
mkdir -p ~/.kaboom
openssl genrsa 2048 > ~/.kaboom/extension-signing-key.pem
```

### Extension ID doesn't match
- Different key = different ID
- Verify key file path in `scripts/release/build-crx.js`

### CRX file corrupted
- Check `extension/manifest.json` exists
- Run `make compile-ts` first to ensure all files are built

## Security Notes

- ✅ Private key is **never** embedded in the CRX file
- ✅ CRX signature is verified by Chrome before installation
- ⚠️ Keep the private key secure — losing it makes it impossible to update existing installations
- ⚠️ Never commit the private key to version control
