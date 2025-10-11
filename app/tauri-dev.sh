#!/bin/bash
# Tauri development with proper macOS SDK paths

# Use system SDK instead of Nix SDK
export DEVELOPER_DIR=/Library/Developer/CommandLineTools
export SDKROOT=/Library/Developer/CommandLineTools/SDKs/MacOSX.sdk

# Point to system clang/cc
export CC=/usr/bin/clang
export CXX=/usr/bin/clang++

# Run tauri dev
npm run tauri:dev
