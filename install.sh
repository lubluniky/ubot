#!/bin/bash
set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
NC='\033[0m' # No Color
BOLD='\033[1m'

# ASCII Art
print_banner() {
    echo -e "${CYAN}"
    cat << 'EOF'
                 ____        __
    __  ______  / __ )____  / /_
   / / / / __ \/ __  / __ \/ __/
  / /_/ / /_/ / /_/ / /_/ / /_
  \__,_/_.___/_____/\____/\__/

  The World's Most Lightweight
     Self-Hosted AI Assistant
EOF
    echo -e "${NC}"
    echo -e "${MAGENTA}  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${MAGENTA}  Shipped to you by ${BOLD}Borkiss${NC}"
    echo -e "${MAGENTA}  https://github.com/lubluniky${NC}"
    echo -e "${MAGENTA}  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
}

# Logging functions
info() { echo -e "${BLUE}[INFO]${NC} $1"; }
success() { echo -e "${GREEN}[OK]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }
step() { echo -e "\n${CYAN}▶${NC} ${BOLD}$1${NC}"; }

# Detect OS
detect_os() {
    if [[ "$OSTYPE" == "linux-gnu"* ]]; then
        if [ -f /etc/os-release ]; then
            . /etc/os-release
            OS=$ID
            OS_VERSION=$VERSION_ID
        fi
        PLATFORM="linux"
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        OS="macos"
        OS_VERSION=$(sw_vers -productVersion)
        PLATFORM="darwin"
    else
        error "Unsupported operating system: $OSTYPE"
    fi
}

# Check if command exists
has_cmd() {
    command -v "$1" &> /dev/null
}

# Check system requirements
check_requirements() {
    step "Checking system requirements..."

    # Check architecture
    ARCH=$(uname -m)
    case $ARCH in
        x86_64|amd64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *) error "Unsupported architecture: $ARCH" ;;
    esac
    success "Architecture: $ARCH"
    success "Platform: $PLATFORM ($OS ${OS_VERSION:-})"

    # Check curl or wget
    if has_cmd curl; then
        DOWNLOADER="curl -fsSL"
        success "curl found"
    elif has_cmd wget; then
        DOWNLOADER="wget -qO-"
        success "wget found"
    else
        error "curl or wget is required"
    fi

    # Check git
    if has_cmd git; then
        success "git found: $(git --version | head -1)"
    else
        warn "git not found - will install"
        NEED_GIT=1
    fi
}

# Wait for Docker to be ready
wait_for_docker() {
    local max_attempts=30
    local attempt=1

    info "Waiting for Docker to start..."
    while [ $attempt -le $max_attempts ]; do
        if docker info &> /dev/null; then
            success "Docker is ready!"
            return 0
        fi
        echo -ne "\r  Waiting... ($attempt/$max_attempts)"
        sleep 2
        ((attempt++))
    done
    echo ""
    return 1
}

# Start Docker Desktop on macOS
start_docker_macos() {
    if docker info &> /dev/null; then
        return 0
    fi

    info "Starting Docker Desktop..."
    open -a Docker 2>/dev/null || open /Applications/Docker.app 2>/dev/null || return 1

    if wait_for_docker; then
        return 0
    else
        error "Docker failed to start. Please start Docker Desktop manually and re-run the installer."
    fi
}

# Install Docker if needed
install_docker() {
    if has_cmd docker; then
        success "Docker found: $(docker --version)"

        # Check if Docker is running
        if docker info &> /dev/null; then
            success "Docker daemon is running"
        else
            warn "Docker daemon is not running"
            if [[ "$PLATFORM" == "darwin" ]]; then
                start_docker_macos
            else
                info "Attempting to start Docker..."
                sudo systemctl start docker || error "Failed to start Docker"
                wait_for_docker || error "Docker failed to start"
                success "Docker started"
            fi
        fi
        return 0
    fi

    step "Installing Docker..."

    if [[ "$PLATFORM" == "darwin" ]]; then
        if has_cmd brew; then
            info "Installing Docker via Homebrew..."
            brew install --cask docker
            info "Starting Docker Desktop..."
            open -a Docker 2>/dev/null || open /Applications/Docker.app 2>/dev/null

            if wait_for_docker; then
                success "Docker installed and running!"
            else
                warn "Docker installed but not yet running."
                info "Please wait for Docker Desktop to fully start, then re-run this installer."
                exit 0
            fi
        else
            info "Homebrew not found. Installing Docker Desktop manually..."

            # Download Docker Desktop DMG
            DOCKER_DMG="Docker.dmg"
            if [[ "$ARCH" == "arm64" ]]; then
                DOCKER_URL="https://desktop.docker.com/mac/main/arm64/Docker.dmg"
            else
                DOCKER_URL="https://desktop.docker.com/mac/main/amd64/Docker.dmg"
            fi

            info "Downloading Docker Desktop..."
            curl -fsSL -o "/tmp/$DOCKER_DMG" "$DOCKER_URL"

            info "Installing Docker Desktop..."
            hdiutil attach "/tmp/$DOCKER_DMG" -quiet
            cp -R "/Volumes/Docker/Docker.app" /Applications/ 2>/dev/null || \
                sudo cp -R "/Volumes/Docker/Docker.app" /Applications/
            hdiutil detach "/Volumes/Docker" -quiet
            rm "/tmp/$DOCKER_DMG"

            info "Starting Docker Desktop..."
            open /Applications/Docker.app

            if wait_for_docker; then
                success "Docker installed and running!"
            else
                warn "Docker installed. Please wait for it to fully start, then re-run this installer."
                exit 0
            fi
        fi
    elif [[ "$PLATFORM" == "linux" ]]; then
        info "Installing Docker via official script..."
        curl -fsSL https://get.docker.com | sh

        # Add user to docker group
        if [ "$EUID" -ne 0 ]; then
            sudo usermod -aG docker "$USER"
            # Use newgrp to apply group changes in current session
            info "Applying docker group permissions..."
        fi

        # Start and enable Docker
        sudo systemctl start docker
        sudo systemctl enable docker

        wait_for_docker || error "Docker failed to start"
        success "Docker installed and started"
    fi
}

# Install git if needed
install_git() {
    if [ "${NEED_GIT:-0}" != "1" ]; then
        return 0
    fi

    step "Installing git..."

    if [[ "$PLATFORM" == "darwin" ]]; then
        xcode-select --install 2>/dev/null || true
    elif [[ "$OS" == "ubuntu" || "$OS" == "debian" ]]; then
        sudo apt-get update && sudo apt-get install -y git
    elif [[ "$OS" == "fedora" ]]; then
        sudo dnf install -y git
    elif [[ "$OS" == "centos" || "$OS" == "rhel" ]]; then
        sudo yum install -y git
    elif [[ "$OS" == "arch" ]]; then
        sudo pacman -S --noconfirm git
    elif [[ "$OS" == "alpine" ]]; then
        sudo apk add git
    else
        error "Please install git manually"
    fi

    success "git installed"
}

# Clone or update repository
setup_repository() {
    step "Setting up uBot repository..."

    INSTALL_DIR="$HOME/.ubot"
    REPO_DIR="$INSTALL_DIR/repo"

    mkdir -p "$INSTALL_DIR"

    if [ -d "$REPO_DIR" ]; then
        info "Updating existing installation..."
        cd "$REPO_DIR"
        git pull origin main || warn "Failed to update, using existing version"
    else
        info "Cloning repository..."
        git clone https://github.com/lubluniky/ubot.git "$REPO_DIR"
        cd "$REPO_DIR"
    fi

    success "Repository ready at $REPO_DIR"
}

# Build Docker image
build_image() {
    step "Building Docker image..."

    cd "$REPO_DIR"

    # Build with version info
    VERSION=$(git describe --tags 2>/dev/null || echo "0.1.0")
    GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
    BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

    info "Version: $VERSION (commit: $GIT_COMMIT)"

    docker build \
        --build-arg VERSION="$VERSION" \
        --build-arg GIT_COMMIT="$GIT_COMMIT" \
        --build-arg BUILD_DATE="$BUILD_DATE" \
        -t ubot:latest \
        -t ubot:"$VERSION" \
        .

    success "Docker image built: ubot:$VERSION"
}

# Create directory structure
setup_dirs() {
    step "Setting up directories..."

    WORKSPACE_DIR="$INSTALL_DIR/workspace"
    mkdir -p "$WORKSPACE_DIR/memory"

    success "Directory structure ready at $INSTALL_DIR"
}

# Create systemd service (Linux only)
create_service() {
    if [[ "$PLATFORM" != "linux" ]]; then
        return 0
    fi

    step "Creating systemd service..."

    SERVICE_FILE="/etc/systemd/system/ubot.service"

    if [ "$EUID" -ne 0 ]; then
        info "Run with sudo to install systemd service, or start manually"
        return 0
    fi

    cat > "$SERVICE_FILE" << EOF
[Unit]
Description=uBot - Lightweight Self-Hosted AI Assistant
After=docker.service
Requires=docker.service

[Service]
Type=simple
Restart=always
RestartSec=5
ExecStart=/usr/bin/docker run --rm --name ubot \\
    -v $INSTALL_DIR:/home/ubot/.ubot \\
    --security-opt no-new-privileges:true \\
    --read-only \\
    --tmpfs /tmp:size=64M \\
    ubot:latest gateway
ExecStop=/usr/bin/docker stop ubot

[Install]
WantedBy=multi-user.target
EOF

    systemctl daemon-reload
    success "Systemd service created"
    info "Enable with: sudo systemctl enable --now ubot"
}

# Create helper scripts
create_scripts() {
    step "Creating helper scripts..."

    BIN_DIR="$HOME/.local/bin"
    mkdir -p "$BIN_DIR"

    # Main ubot command
    cat > "$BIN_DIR/ubot" << 'SCRIPT'
#!/bin/bash
UBOT_DIR="$HOME/.ubot"

case "${1:-help}" in
    start|gateway)
        echo "Starting uBot gateway..."
        docker run -d --rm --name ubot \
            -v "$UBOT_DIR:/home/ubot/.ubot" \
            --security-opt no-new-privileges:true \
            --read-only \
            --tmpfs /tmp:size=64M \
            ubot:latest gateway
        echo "uBot is running. Check logs with: ubot logs"
        ;;
    stop)
        echo "Stopping uBot..."
        docker stop ubot 2>/dev/null || echo "uBot is not running"
        ;;
    restart)
        $0 stop
        sleep 1
        $0 start
        ;;
    logs)
        docker logs -f ubot 2>/dev/null || echo "uBot is not running"
        ;;
    status)
        docker run --rm \
            -v "$UBOT_DIR:/home/ubot/.ubot" \
            ubot:latest status
        ;;
    chat)
        shift
        docker run -it --rm \
            -v "$UBOT_DIR:/home/ubot/.ubot" \
            ubot:latest agent "$@"
        ;;
    setup)
        docker run -it --rm \
            -v "$UBOT_DIR:/home/ubot/.ubot" \
            ubot:latest setup
        ;;
    config)
        ${EDITOR:-nano} "$UBOT_DIR/config.json"
        ;;
    update)
        echo "Updating uBot..."
        cd "$UBOT_DIR/repo" && git pull
        docker build -t ubot:latest .
        echo "Update complete. Restart with: ubot restart"
        ;;
    destroy)
        echo ""
        echo -e "\033[1;31m⚠️  WARNING: This will permanently delete uBot and all its data!\033[0m"
        echo ""
        echo "This includes:"
        echo "  - Docker container and image"
        echo "  - Configuration (~/.ubot/config.json)"
        echo "  - Workspace and memory (~/.ubot/workspace/)"
        echo "  - Session history (~/.ubot/sessions/)"
        echo "  - Repository (~/.ubot/repo/)"
        echo ""
        read -p "Are you sure? Type 'destroy' to confirm: " confirm
        if [ "$confirm" = "destroy" ]; then
            echo ""
            echo "Stopping uBot containers..."
            docker stop ubot 2>/dev/null || true
            docker stop ubot-sandboxed 2>/dev/null || true
            docker rm ubot-sandboxed 2>/dev/null || true
            echo "Removing Docker images..."
            docker images --filter=reference='ubot' -q | xargs docker rmi 2>/dev/null || true
            echo "Removing systemd service (if any)..."
            if [ -f /etc/systemd/system/ubot.service ]; then
                sudo systemctl stop ubot 2>/dev/null || true
                sudo systemctl disable ubot 2>/dev/null || true
                sudo rm -f /etc/systemd/system/ubot.service
                sudo systemctl daemon-reload 2>/dev/null || true
            fi
            echo "Cleaning PATH entries from shell configs..."
            for rcfile in "$HOME/.zshrc" "$HOME/.bashrc" "$HOME/.bash_profile" "$HOME/.profile"; do
                if [ -f "$rcfile" ]; then
                    sed -i.bak -e '/# Added by uBot installer/d' -e '/export PATH=.*\.local\/bin/d' "$rcfile" 2>/dev/null || \
                        sed -i '' -e '/# Added by uBot installer/d' -e '/export PATH=.*\.local\/bin/d' "$rcfile" 2>/dev/null || true
                    rm -f "${rcfile}.bak"
                fi
            done
            echo "Removing data directory..."
            rm -rf "$UBOT_DIR"
            echo "Removing ubot command..."
            rm -f "$HOME/.local/bin/ubot"
            echo ""
            echo -e "\033[0;32m✓ uBot has been completely removed.\033[0m"
            echo "Thank you for using uBot!"
        else
            echo "Aborted."
        fi
        ;;
    *)
        echo "uBot - The World's Most Lightweight Self-Hosted AI Assistant"
        echo ""
        echo "Usage: ubot <command>"
        echo ""
        echo "Commands:"
        echo "  start     Start the gateway (Telegram, etc.)"
        echo "  stop      Stop the gateway"
        echo "  restart   Restart the gateway"
        echo "  logs      Show gateway logs"
        echo "  status    Show configuration status"
        echo "  chat      Interactive chat mode"
        echo "  chat -m   Send a single message"
        echo "  setup     Run setup wizard"
        echo "  config    Edit configuration"
        echo "  update    Update to latest version"
        echo "  destroy   Remove uBot completely"
        echo ""
        echo "Examples:"
        echo "  ubot start"
        echo "  ubot chat -m \"Hello!\""
        echo "  ubot logs"
        ;;
esac
SCRIPT

    chmod +x "$BIN_DIR/ubot"

    success "Created ubot command at $BIN_DIR/ubot"

    # Add to PATH automatically
    add_to_path
}

# Add ~/.local/bin to PATH in shell config
add_to_path() {
    BIN_DIR="$HOME/.local/bin"
    PATH_EXPORT='export PATH="$HOME/.local/bin:$PATH"'

    # Check if already in PATH
    if [[ ":$PATH:" == *":$BIN_DIR:"* ]]; then
        success "PATH already configured"
        return 0
    fi

    step "Adding ubot to PATH..."

    # Detect shell and config file
    SHELL_NAME=$(basename "$SHELL")
    ADDED=0

    case "$SHELL_NAME" in
        zsh)
            SHELL_RC="$HOME/.zshrc"
            ;;
        bash)
            # On macOS, bash uses .bash_profile for login shells
            if [[ "$PLATFORM" == "darwin" ]]; then
                SHELL_RC="$HOME/.bash_profile"
            else
                SHELL_RC="$HOME/.bashrc"
            fi
            ;;
        *)
            SHELL_RC="$HOME/.profile"
            ;;
    esac

    # Check if already added to config
    if [ -f "$SHELL_RC" ] && grep -q '\.local/bin' "$SHELL_RC" 2>/dev/null; then
        success "PATH export already in $SHELL_RC"
        ADDED=1
    else
        # Add to shell config
        echo "" >> "$SHELL_RC"
        echo "# Added by uBot installer" >> "$SHELL_RC"
        echo "$PATH_EXPORT" >> "$SHELL_RC"
        success "Added PATH to $SHELL_RC"
        ADDED=1
    fi

    # Also add to .profile for login shells (Linux)
    if [[ "$PLATFORM" == "linux" ]] && [ "$SHELL_RC" != "$HOME/.profile" ]; then
        if [ -f "$HOME/.profile" ] && ! grep -q '\.local/bin' "$HOME/.profile" 2>/dev/null; then
            echo "" >> "$HOME/.profile"
            echo "# Added by uBot installer" >> "$HOME/.profile"
            echo "$PATH_EXPORT" >> "$HOME/.profile"
        fi
    fi

    # Export for current session
    export PATH="$HOME/.local/bin:$PATH"
}

# Print completion message and prompt for setup
print_complete() {
    echo ""
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}  Installation Complete!${NC}"
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    echo -e "  ${BOLD}Next step:${NC} Run ${CYAN}ubot setup${NC} to configure your assistant."
    echo ""
    echo -e "  ${BOLD}Other commands:${NC}"
    echo ""
    echo -e "    ${CYAN}ubot start${NC}     - Start the gateway"
    echo -e "    ${CYAN}ubot chat${NC}      - Interactive chat"
    echo -e "    ${CYAN}ubot status${NC}    - Check configuration"
    echo ""
    echo -e "${MAGENTA}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${MAGENTA}  Shipped to you by ${BOLD}Borkiss${NC}"
    echo -e "${MAGENTA}  https://github.com/lubluniky${NC}"
    echo -e "${MAGENTA}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""

    # Offer to run setup now
    echo -e "  ${YELLOW}Would you like to run setup now?${NC}"
    read -p "  Run 'ubot setup'? [Y/n] " -n 1 -r < /dev/tty
    echo ""
    if [[ ! $REPLY =~ ^[Nn]$ ]]; then
        echo ""
        exec ubot setup
    else
        echo ""
        echo -e "  ${CYAN}Restarting shell to apply PATH...${NC}"
        echo ""
    fi
}

# Select installation mode
select_install_mode() {
    step "Select installation mode..."
    echo ""
    echo "  1) Docker (recommended) - Isolated, secure, easy updates"
    echo "  2) Native - Direct Go binary, no Docker needed"
    echo ""
    read -p "  Select mode [1-2] (default: 1): " mode_choice < /dev/tty

    case "${mode_choice:-1}" in
        2) INSTALL_MODE="native" ;;
        *) INSTALL_MODE="docker" ;;
    esac

    success "Installation mode: $INSTALL_MODE"
}

# Install Go if needed (for native mode)
install_go() {
    if has_cmd go; then
        GO_VERSION=$(go version | awk '{print $3}')
        success "Go found: $GO_VERSION"
        return 0
    fi

    step "Installing Go..."

    GO_VERSION="1.23.0"
    if [[ "$PLATFORM" == "darwin" ]]; then
        if has_cmd brew; then
            brew install go
        else
            curl -fsSL "https://go.dev/dl/go${GO_VERSION}.darwin-${ARCH}.tar.gz" -o /tmp/go.tar.gz
            sudo tar -C /usr/local -xzf /tmp/go.tar.gz
            rm /tmp/go.tar.gz
            export PATH=$PATH:/usr/local/go/bin
        fi
    elif [[ "$PLATFORM" == "linux" ]]; then
        curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz" -o /tmp/go.tar.gz
        sudo tar -C /usr/local -xzf /tmp/go.tar.gz
        rm /tmp/go.tar.gz
        export PATH=$PATH:/usr/local/go/bin
    fi

    if has_cmd go; then
        success "Go installed: $(go version | awk '{print $3}')"
    else
        error "Failed to install Go"
    fi
}

# Build native binary
build_native() {
    step "Building native binary..."

    cd "$REPO_DIR"

    # Build with version info
    VERSION=$(git describe --tags 2>/dev/null || echo "0.1.0")
    GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
    BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

    info "Version: $VERSION (commit: $GIT_COMMIT)"

    go build -ldflags="-s -w \
        -X 'github.com/hkuds/ubot/cmd/ubot/cmd.Version=${VERSION}' \
        -X 'github.com/hkuds/ubot/cmd/ubot/cmd.GitCommit=${GIT_COMMIT}' \
        -X 'github.com/hkuds/ubot/cmd/ubot/cmd.BuildDate=${BUILD_DATE}'" \
        -o "$INSTALL_DIR/bin/ubot" ./cmd/ubot/

    success "Binary built: $INSTALL_DIR/bin/ubot"
}

# Create helper scripts for native mode
create_native_scripts() {
    step "Creating helper scripts (native mode)..."

    BIN_DIR="$HOME/.local/bin"
    mkdir -p "$BIN_DIR"

    # Symlink or wrapper
    cat > "$BIN_DIR/ubot" << SCRIPT
#!/bin/bash
UBOT_DIR="\$HOME/.ubot"
UBOT_BIN="\$UBOT_DIR/bin/ubot"

case "\${1:-help}" in
    start|gateway)
        echo "Starting uBot gateway..."
        nohup "\$UBOT_BIN" gateway > "\$UBOT_DIR/ubot.log" 2>&1 &
        echo \$! > "\$UBOT_DIR/ubot.pid"
        echo "uBot is running (PID: \$(cat \$UBOT_DIR/ubot.pid)). Check logs with: ubot logs"
        ;;
    stop)
        echo "Stopping uBot..."
        if [ -f "\$UBOT_DIR/ubot.pid" ]; then
            kill \$(cat "\$UBOT_DIR/ubot.pid") 2>/dev/null && rm "\$UBOT_DIR/ubot.pid"
            echo "Stopped."
        else
            echo "uBot is not running"
        fi
        ;;
    restart)
        \$0 stop
        sleep 1
        \$0 start
        ;;
    logs)
        tail -f "\$UBOT_DIR/ubot.log" 2>/dev/null || echo "No logs found"
        ;;
    status)
        "\$UBOT_BIN" status
        ;;
    chat)
        shift
        "\$UBOT_BIN" agent "\$@"
        ;;
    setup)
        "\$UBOT_BIN" setup
        ;;
    config)
        \${EDITOR:-nano} "\$UBOT_DIR/config.json"
        ;;
    update)
        echo "Updating uBot..."
        cd "\$UBOT_DIR/repo" && git pull
        go build -o "\$UBOT_BIN" ./cmd/ubot/
        echo "Update complete. Restart with: ubot restart"
        ;;
    destroy)
        echo ""
        echo -e "\033[1;31m⚠️  WARNING: This will permanently delete uBot and all its data!\033[0m"
        echo ""
        echo "This includes:"
        echo "  - uBot binary and repository"
        echo "  - Configuration (~/.ubot/config.json)"
        echo "  - Workspace and memory (~/.ubot/workspace/)"
        echo "  - Session history"
        echo ""
        read -p "Are you sure? Type 'destroy' to confirm: " confirm
        if [ "\$confirm" = "destroy" ]; then
            \$0 stop 2>/dev/null
            if [ -f /etc/systemd/system/ubot.service ]; then
                echo "Removing systemd service..."
                sudo systemctl stop ubot 2>/dev/null || true
                sudo systemctl disable ubot 2>/dev/null || true
                sudo rm -f /etc/systemd/system/ubot.service
                sudo systemctl daemon-reload 2>/dev/null || true
            fi
            echo "Cleaning PATH entries from shell configs..."
            for rcfile in "\$HOME/.zshrc" "\$HOME/.bashrc" "\$HOME/.bash_profile" "\$HOME/.profile"; do
                if [ -f "\$rcfile" ]; then
                    sed -i.bak -e '/# Added by uBot installer/d' -e '/export PATH=.*\.local\/bin/d' "\$rcfile" 2>/dev/null || \
                        sed -i '' -e '/# Added by uBot installer/d' -e '/export PATH=.*\.local\/bin/d' "\$rcfile" 2>/dev/null || true
                    rm -f "\${rcfile}.bak"
                fi
            done
            rm -rf "\$UBOT_DIR"
            rm -f "\$HOME/.local/bin/ubot"
            echo ""
            echo -e "\033[0;32m✓ uBot has been completely removed.\033[0m"
            echo "Thank you for using uBot!"
        else
            echo "Aborted."
        fi
        ;;
    *)
        echo "uBot - The World's Most Lightweight Self-Hosted AI Assistant"
        echo ""
        echo "Usage: ubot <command>"
        echo ""
        echo "Commands:"
        echo "  start     Start the gateway (Telegram, etc.)"
        echo "  stop      Stop the gateway"
        echo "  restart   Restart the gateway"
        echo "  logs      Show gateway logs"
        echo "  status    Show configuration status"
        echo "  chat      Interactive chat mode"
        echo "  chat -m   Send a single message"
        echo "  setup     Run setup wizard"
        echo "  config    Edit configuration"
        echo "  update    Update to latest version"
        echo "  destroy   Remove uBot completely"
        echo ""
        ;;
esac
SCRIPT

    chmod +x "$BIN_DIR/ubot"
    success "Created ubot command at $BIN_DIR/ubot"

    # Add to PATH automatically
    add_to_path
}

# Main installation flow
main() {
    print_banner

    detect_os
    check_requirements
    select_install_mode
    install_git

    if [[ "$INSTALL_MODE" == "docker" ]]; then
        install_docker
        setup_repository
        build_image
        setup_dirs
        create_service
        create_scripts
    else
        install_go
        setup_repository
        build_native
        setup_dirs
        create_native_scripts
    fi

    print_complete
}

# Run main
main "$@"

# Restart shell to apply PATH changes
exec $SHELL
