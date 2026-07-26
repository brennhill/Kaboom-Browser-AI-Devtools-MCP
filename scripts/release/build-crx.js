#!/usr/bin/env node
// Purpose: Automate build-crx.js workflow behavior for repository tooling.
// Why: Keeps repetitive maintenance and verification steps deterministic.
// Docs: docs/DEVELOPMENT.md

import fs from 'fs'
import path from 'path'
// Node built-in import, not hiding a core module
import crypto from 'crypto'
import { promisify } from 'util'
import { exec as execCallback } from 'child_process'
// fileURLToPath available if needed for __dirname resolution

const exec = promisify(execCallback)

// Get version from VERSION file
const VERSION = fs.readFileSync('./VERSION', 'utf-8').trim()
const KEY_FILE = path.resolve(process.env.HOME, '.kaboom/extension-signing-key.pem')
const EXTENSION_DIR = './extension'
const BUILD_DIR = './dist'
const OUTPUT_CRX = path.join(BUILD_DIR, `kaboom-extension-v${VERSION}.crx`)
const TEMP_ZIP = path.join(BUILD_DIR, `.kaboom-temp-${Date.now()}.zip`)

async function buildCRX() {
  try {
    // Check for key file

    if (!fs.existsSync(KEY_FILE)) {
      console.error(`❌ Private key not found at ${KEY_FILE}`)
      // CLI script exits with error status on fatal failure
      process.exit(1)
    }

    // Ensure build dir exists
    if (!fs.existsSync(BUILD_DIR)) {
      fs.mkdirSync(BUILD_DIR, { recursive: true })
    }

    // Try to use Chrome's native pack-extension FIRST (most reliable - Chrome's own implementation)
    const chromeCommand = getChromeCommand()
    if (chromeCommand) {
      console.log('🔧 Using Chrome native packing (most reliable)...')
      try {
        await exec(
          `${chromeCommand} --pack-extension="${path.resolve(EXTENSION_DIR)}" --pack-extension-key="${KEY_FILE}"`
        )
        const nativeCrx = path.join(EXTENSION_DIR + '.crx')
        if (fs.existsSync(nativeCrx)) {
          fs.copyFileSync(nativeCrx, OUTPUT_CRX)
          fs.unlinkSync(nativeCrx)

          // Extract and display extension ID

          const data = fs.readFileSync(OUTPUT_CRX)
          const headerLen = data.readUInt32LE(8)
          const headerProto = data.slice(12, 12 + headerLen)
          let offset = 1
          let length1 = 0
          let shift = 0
          while (offset < headerProto.length) {
            const byte = headerProto[offset]
            length1 |= (byte & 0x7f) << shift
            offset++
            if ((byte & 0x80) === 0) break
            shift += 7
          }
          const signedHeaderData = headerProto.slice(offset, offset + length1)
          const hash = crypto.createHash('sha256').update(signedHeaderData).digest()
          const extensionId = toBase32(hash).slice(0, 32)

          console.log(`\n✨ CRX file created: ${OUTPUT_CRX}`)
          console.log(`📊 File size: ${(data.length / 1024).toFixed(1)} KB`)
          console.log(`📦 Extension ID: ${extensionId}`)
          return
        }
      } catch (_err) {
        console.log('⚠️  Chrome native packing failed, trying crx tool...')
      }
    }

    // Try to use crx (Rust tool) as fallback
    try {
      console.log('🔧 Using crx (Rust tool) for packing...')
      await exec(`crx pack "${path.resolve(EXTENSION_DIR)}" -o "${OUTPUT_CRX}" -k "${KEY_FILE}"`)

      if (fs.existsSync(OUTPUT_CRX)) {
        const data = fs.readFileSync(OUTPUT_CRX)
        const headerLen = data.readUInt32LE(8)
        const headerProto = data.slice(12, 12 + headerLen)
        let offset = 1
        let length1 = 0
        let shift = 0
        while (offset < headerProto.length) {
          const byte = headerProto[offset]
          length1 |= (byte & 0x7f) << shift
          offset++
          if ((byte & 0x80) === 0) break
          shift += 7
        }
        const signedHeaderData = headerProto.slice(offset, offset + length1)
        const hash = crypto.createHash('sha256').update(signedHeaderData).digest()
        const extensionId = toBase32(hash).slice(0, 32)

        console.log(`\n✨ CRX file created: ${OUTPUT_CRX}`)
        console.log(`📊 File size: ${(data.length / 1024).toFixed(1)} KB`)
        console.log(`📦 Extension ID: ${extensionId}`)
        return
      }
    } catch (_err) {
      console.log('⚠️  crx tool failed, falling back to manual method')
    }

    console.log('📦 Creating extension zip...')

    // Create zip using system zip command
    try {
      await exec(`cd ${EXTENSION_DIR} && zip -q -r "../${TEMP_ZIP}" \
        .`)
    } catch (err) {
      console.error('❌ Failed to create zip:', err.message)
      // CLI script exits with error status
      process.exit(1)
    }

    if (!fs.existsSync(TEMP_ZIP)) {
      console.error('❌ Failed to create extension zip')
      process.exit(1)
    }

    console.log('🔐 Reading private key...')

    const keyContent = fs.readFileSync(KEY_FILE, 'utf-8')

    // Extract public key and compute extension ID
    console.log('🔑 Computing extension ID...')
    const publicKeyPem = crypto.createPublicKey({
      key: keyContent,
      format: 'pem'
    })

    const publicKeyDer = publicKeyPem.export({ format: 'der', type: 'spki' })
    const hash = crypto.createHash('sha256').update(publicKeyDer).digest()

    // Convert hash to Chrome extension ID format (base32)
    const extensionId = toBase32(hash).slice(0, 32)

    console.log(`✅ Extension ID: ${extensionId}`)

    // Read and sign the header (not the zip!)
    console.log('✍️  Signing extension...')

    const zipData = fs.readFileSync(TEMP_ZIP)

    // Create the signed header data first
    const signedHeaderData = createSignedHeader(publicKeyDer)

    // Sign the signed header data (not the zip!)
    const signer = crypto.createSign('sha256')
    signer.update(signedHeaderData)
    const signature = signer.sign(keyContent)

    // Build CRX3 file format
    // Magic: "Cr24" (0x4372323434)
    const magic = Buffer.from('Cr24', 'ascii')
    const version = Buffer.alloc(4)
    version.writeUInt32LE(3) // CRX3

    // Create protobuf header (simplified)
    const headerProto = createHeaderProto(publicKeyDer, signature, signedHeaderData)
    const headerLen = Buffer.alloc(4)
    headerLen.writeUInt32LE(headerProto.length)

    const crxBuffer = Buffer.concat([magic, version, headerLen, headerProto, zipData])

    // Write CRX file

    fs.writeFileSync(OUTPUT_CRX, crxBuffer)

    // Cleanup

    fs.unlinkSync(TEMP_ZIP)

    console.log(`\n✨ CRX file created: ${OUTPUT_CRX}`)
    console.log(`📊 File size: ${(crxBuffer.length / 1024).toFixed(1)} KB`)
    console.log(`\n📋 Installation instructions:`)
    console.log(`1. Open Chrome and go to chrome://extensions/`)
    console.log(`2. Enable "Developer mode" (top right)`)
    console.log(`3. Drag and drop the ${OUTPUT_CRX} file into the page`)
    console.log(`\n🔗 Distribution URL: https://gokaboom.dev/downloads/kaboom-extension-v${VERSION}.crx`)
    console.log(`📦 Extension ID: ${extensionId}`)
  } catch (err) {
    console.error('❌ Error building CRX:', err.message)

    if (fs.existsSync(TEMP_ZIP)) fs.unlinkSync(TEMP_ZIP)
    process.exit(1)
  }
}

function createHeaderProto(publicKey, signature, signedHeaderData) {
  // Simplified protobuf encoding for CRXv3
  // Field 1: signed_header_data (bytes)
  // Field 2: signature (bytes)

  let proto = Buffer.alloc(0)

  // Encode field 1 (signed_header_data)
  proto = Buffer.concat([
    proto,
    encodeVarint((1 << 3) | 2), // field 1, wire type 2 (length-delimited)
    encodeVarint(signedHeaderData.length),
    signedHeaderData
  ])

  // Encode field 2 (signature)
  proto = Buffer.concat([
    proto,
    encodeVarint((2 << 3) | 2), // field 2, wire type 2 (length-delimited)
    encodeVarint(signature.length),
    signature
  ])

  return proto
}

function createSignedHeader(publicKey) {
  // CRXv3 signed_header contains the public key
  // Field 1: key_pairs (repeated)

  let data = Buffer.alloc(0)

  // Encode key_pairs[0] (first key pair)
  data = Buffer.concat([
    data,
    encodeVarint((1 << 3) | 2), // field 1, wire type 2
    encodeVarint(publicKey.length),
    publicKey
  ])

  return data
}

function toBase32(buf) {
  const alphabet = 'abcdefghijklmnopqrstuvwxyz234567'
  let result = ''
  let bits = 0
  let value = 0

  for (let i = 0; i < buf.length; i++) {
    value = (value << 8) | buf[i]
    bits += 8
    while (bits >= 5) {
      bits -= 5
      result += alphabet[(value >> bits) & 31]
    }
  }

  if (bits > 0) {
    result += alphabet[(value << (5 - bits)) & 31]
  }

  return result
}

function encodeVarint(value) {
  const bytes = []
  while ((value & 0xffffff80) !== 0) {
    bytes.push(((value & 0x7f) | 0x80) & 0xff)
    value >>>= 7
  }
  bytes.push(value & 0x7f)
  return Buffer.from(bytes)
}

function getChromeCommand() {
  // Try common Chrome/Chromium locations (macOS first, most common)
  const candidates = [
    '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome', // macOS
    '/Applications/Brave Browser.app/Contents/MacOS/Brave Browser', // Brave (macOS)
    '/Applications/Chromium.app/Contents/MacOS/Chromium', // Chromium (macOS)
    'google-chrome', // Linux
    'google-chrome-stable', // Linux
    'chromium', // Linux
    'chromium-browser', // Linux
    'chrome', // Linux/generic
    'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe', // Windows
    'C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe' // Windows 32-bit
  ]

  for (const cmd of candidates) {
    try {
      // For macOS paths with spaces, check if file exists directly
      if (cmd.startsWith('/Applications')) {
        if (fs.existsSync(cmd)) {
          return cmd
        }
      } else {
        // For other paths, use execSync
        execCallback(`"${cmd}" --version`, { stdio: 'pipe' })
        return cmd
      }
    } catch (_e) {
      // Not found, try next
    }
  }

  return null
}

buildCRX().catch((err) => {
  console.error(err)
  process.exit(1)
})
