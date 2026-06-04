#!/bin/bash

echo "Opsætter tailTUI desktop-integration for Omarchy..."

# Definer standard stier
ICON_DIR="$HOME/.local/share/icons/hicolor/256x256/apps"
APP_DIR="$HOME/.local/share/applications"

# Sikr at mapperne eksisterer
mkdir -p "$ICON_DIR"
mkdir -p "$APP_DIR"

# Kopier filer fra omarchy-mappen
cp assets/omarchy/tailtui-icon.png "$ICON_DIR/"
cp assets/omarchy/tailtui.desktop "$APP_DIR/"

# Opdater ikon-cachen, så app-launcheren fatter beskeden med det samme
if command -v gtk-update-icon-cache &> /dev/null; then
    gtk-update-icon-cache "$HOME/.local/share/icons/hicolor/" -f -t
fi

echo "✅ Succes! tailTUI ligger nu i din app-launcher."
