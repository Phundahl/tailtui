#!/bin/bash

echo "Setting up tailTUI desktop integration for Omarchy/Linux..."

# Define standard paths
ICON_DIR="$HOME/.local/share/icons/hicolor/256x256/apps"
APP_DIR="$HOME/.local/share/applications"

# Locate the tailtui binary
# If it's available in the system PATH, use that. Otherwise, fallback to Go's default bin directory.
TARGET_PATH=$(which tailtui 2>/dev/null)
if [ -z "$TARGET_PATH" ]; then
    TARGET_PATH="$HOME/go/bin/tailtui"
fi

# Ensure directories exist
mkdir -p "$ICON_DIR"
mkdir -p "$APP_DIR"

# Copy the icon
cp assets/omarchy/tailtui-icon.png "$ICON_DIR/"

# Copy the desktop file and replace the placeholder with the correct absolute path
sed "s|__TAILTUI_PATH__|$TARGET_PATH|g" assets/omarchy/tailtui.desktop > "$APP_DIR/tailtui.desktop"

# Update the icon cache silently (ignores errors on minimal systems missing index.theme)
if command -v gtk-update-icon-cache &> /dev/null; then
    gtk-update-icon-cache "$HOME/.local/share/icons/hicolor/" -f -t 2>/dev/null || true
fi

echo "✅ Success! tailTUI is now available in your app launcher (Path: $TARGET_PATH)."
