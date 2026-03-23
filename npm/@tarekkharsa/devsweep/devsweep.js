#!/usr/bin/env node

const os = require('os');
const path = require('path');
const fs = require('fs');
const https = require('https');
const http = require('http');
const { execSync, spawn } = require('child_process');

const REPO = 'TarekKharsa/devsweep';
const BINARY_NAME = 'devsweep';

function getPlatform() {
  const platform = os.platform();
  if (platform === 'darwin') return 'darwin';
  if (platform === 'linux') return 'linux';
  throw new Error(`Unsupported platform: ${platform}`);
}

function getArch() {
  const arch = os.arch();
  if (arch === 'x64') return 'amd64';
  if (arch === 'arm64') return 'arm64';
  throw new Error(`Unsupported architecture: ${arch}`);
}

function getBinaryUrl(version) {
  const platform = getPlatform();
  const arch = getArch();
  const versionWithoutV = version.replace(/^v/, '');
  return `https://github.com/${REPO}/releases/download/${version}/${BINARY_NAME}_${versionWithoutV}_${platform}_${arch}.tar.gz`;
}

function getLocalBinaryPath() {
  return path.join(os.homedir(), '.devsweep-bin', BINARY_NAME);
}

function downloadFile(url, dest) {
  return new Promise((resolve, reject) => {
    const file = fs.createWriteStream(dest);
    const protocol = url.startsWith('https') ? https : http;
    
    protocol.get(url, (response) => {
      if (response.statusCode === 302 || response.statusCode === 301) {
        downloadFile(response.headers.location, dest).then(resolve).catch(reject);
        return;
      }
      
      if (response.statusCode !== 200) {
        reject(new Error(`Failed to download: ${response.statusCode}`));
        return;
      }
      
      response.pipe(file);
      file.on('finish', () => {
        file.close();
        resolve();
      });
    }).on('error', (err) => {
      fs.unlink(dest, () => {});
      reject(err);
    });
  });
}

async function installBinary() {
  const versionUrl = `https://api.github.com/repos/${REPO}/releases/latest`;
  
  console.log('Fetching latest version...');
  const versionResponse = await new Promise((resolve, reject) => {
    https.get(versionUrl, { headers: { 'User-Agent': 'devsweep-npm' } }, resolve).on('error', reject);
  });
  
  const versionData = JSON.parse(await new Promise((resolve, reject) => {
    let data = '';
    versionResponse.on('data', chunk => data += chunk);
    versionResponse.on('end', () => resolve(data));
    versionResponse.on('error', reject);
  }));
  
  const version = versionData.tag_name;
  const url = getBinaryUrl(version);
  
  console.log(`Installing DevSweep ${version}...`);
  
  const installDir = path.dirname(getLocalBinaryPath());
  if (!fs.existsSync(installDir)) {
    fs.mkdirSync(installDir, { recursive: true });
  }
  
  const tmpFile = path.join(os.tmpdir(), `devsweep-${Date.now()}.tar.gz`);
  
  try {
    await downloadFile(url, tmpFile);
    
    // Extract tar.gz
    const tarPath = path.join(os.tmpdir(), 'devsweep-bin');
    if (fs.existsSync(tarPath)) {
      fs.rmSync(tarPath, { recursive: true });
    }
    fs.mkdirSync(tarPath, { recursive: true });
    
    execSync(`tar -xzf "${tmpFile}" -C "${tarPath}"`, { stdio: 'pipe' });
    
    const extractedBinary = path.join(tarPath, BINARY_NAME);
    fs.copyFileSync(extractedBinary, getLocalBinaryPath());
    fs.chmodSync(getLocalBinaryPath(), 0o755);
    
    console.log(`Installed to ${getLocalBinaryPath()}`);
  } finally {
    fs.unlink(tmpFile, () => {});
  }
}

async function main() {
  const args = process.argv.slice(2);
  
  // Check if this is just --version or -v
  if (args[0] === '--version' || args[0] === '-v') {
    console.log('DevSweep npm wrapper');
    console.log('Actual version managed by the binary');
    return;
  }
  
  const binaryPath = getLocalBinaryPath();
  
  // Install if needed
  if (!fs.existsSync(binaryPath)) {
    await installBinary();
  }
  
  // Run the actual command
  const child = spawn(binaryPath, args, {
    stdio: 'inherit',
    env: { ...process.env }
  });
  
  child.on('exit', (code) => {
    process.exit(code || 0);
  });
}

main().catch(err => {
  console.error(err.message);
  process.exit(1);
});
