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
                info "Please start Docker Desktop and re-run this installer"
                exit 1
            else
                info "Attempting to start Docker..."
                sudo systemctl start docker || error "Failed to start Docker"
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
            info "Please start Docker Desktop from Applications, then re-run this installer"
            exit 0
        else
            error "Please install Docker Desktop from https://docker.com/products/docker-desktop"
        fi
    elif [[ "$PLATFORM" == "linux" ]]; then
        info "Installing Docker via official script..."
        curl -fsSL https://get.docker.com | sh

        # Add user to docker group
        if [ "$EUID" -ne 0 ]; then
            sudo usermod -aG docker "$USER"
            warn "You've been added to the docker group. Please log out and back in, then re-run this installer."
            exit 0
        fi

        # Start and enable Docker
        sudo systemctl start docker
        sudo systemctl enable docker
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

# Create configuration
setup_config() {
    step "Setting up configuration..."

    CONFIG_FILE="$INSTALL_DIR/config.json"
    WORKSPACE_DIR="$INSTALL_DIR/workspace"

    mkdir -p "$WORKSPACE_DIR/memory"

    if [ -f "$CONFIG_FILE" ]; then
        info "Configuration already exists at $CONFIG_FILE"
        read -p "Do you want to reconfigure? [y/N] " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            return 0
        fi
    fi

    echo ""
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BOLD}  Provider Setup${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    echo "  1) OpenRouter (recommended - access to Claude, GPT-4, Llama)"
    echo "  2) OpenAI (GPT-4)"
    echo "  3) Anthropic (Claude)"
    echo "  4) Ollama (local models, free)"
    echo "  5) Skip for now"
    echo ""
    read -p "  Select provider [1-5]: " provider_choice

    case $provider_choice in
        1)
            echo ""
            echo -e "  Get your API key at: ${BLUE}https://openrouter.ai/keys${NC}"
            read -p "  Enter OpenRouter API key: " api_key
            PROVIDER_CONFIG='"openrouter": { "apiKey": "'"$api_key"'" }'
            MODEL="anthropic/claude-sonnet-4-20250514"
            ;;
        2)
            echo ""
            echo -e "  Get your API key at: ${BLUE}https://platform.openai.com/api-keys${NC}"
            read -p "  Enter OpenAI API key: " api_key
            PROVIDER_CONFIG='"openai": { "apiKey": "'"$api_key"'" }'
            MODEL="gpt-4o"
            ;;
        3)
            echo ""
            echo -e "  Get your API key at: ${BLUE}https://console.anthropic.com${NC}"
            read -p "  Enter Anthropic API key: " api_key
            PROVIDER_CONFIG='"anthropic": { "apiKey": "'"$api_key"'" }'
            MODEL="claude-sonnet-4-20250514"
            ;;
        4)
            info "Make sure Ollama is running: ollama serve"
            PROVIDER_CONFIG='"ollama": { "apiBase": "http://host.docker.internal:11434/v1" }'
            MODEL="llama3.2"
            ;;
        *)
            warn "Skipping provider setup. Edit $CONFIG_FILE later."
            PROVIDER_CONFIG='"openrouter": { "apiKey": "" }'
            MODEL="anthropic/claude-sonnet-4-20250514"
            ;;
    esac

    # Telegram setup
    echo ""
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BOLD}  Telegram Setup (optional)${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    read -p "  Setup Telegram bot? [y/N] " -n 1 -r
    echo

    TELEGRAM_ENABLED="false"
    TELEGRAM_TOKEN=""
    TELEGRAM_ALLOW=""

    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo -e "  Create a bot via ${BLUE}@BotFather${NC} on Telegram"
        read -p "  Enter bot token: " TELEGRAM_TOKEN
        read -p "  Enter your Telegram user ID (from @userinfobot): " TELEGRAM_ALLOW
        TELEGRAM_ENABLED="true"
    fi

    # Write config
    cat > "$CONFIG_FILE" << EOF
{
  "agents": {
    "defaults": {
      "model": "$MODEL",
      "maxTokens": 4096,
      "temperature": 0.7,
      "maxToolIterations": 10
    }
  },
  "providers": {
    $PROVIDER_CONFIG
  },
  "channels": {
    "telegram": {
      "enabled": $TELEGRAM_ENABLED,
      "token": "$TELEGRAM_TOKEN",
      "allowFrom": ["$TELEGRAM_ALLOW"]
    }
  },
  "tools": {
    "exec": {
      "timeout": 30,
      "restrictToWorkspace": true
    }
  }
}
EOF

    success "Configuration saved to $CONFIG_FILE"
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
            echo "Stopping uBot..."
            docker stop ubot 2>/dev/null || true
            echo "Removing Docker image..."
            docker rmi ubot:latest 2>/dev/null || true
            docker rmi ubot 2>/dev/null || true
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

    # Add to PATH if needed
    if [[ ":$PATH:" != *":$BIN_DIR:"* ]]; then
        warn "Add this to your ~/.bashrc or ~/.zshrc:"
        echo -e "  ${CYAN}export PATH=\"\$HOME/.local/bin:\$PATH\"${NC}"
    fi
}

# Print completion message
print_complete() {
    echo ""
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${GREEN}  Installation Complete!${NC}"
    echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    echo -e "  ${BOLD}Quick Start:${NC}"
    echo ""
    echo -e "    ${CYAN}ubot start${NC}     - Start the gateway"
    echo -e "    ${CYAN}ubot chat${NC}      - Interactive chat"
    echo -e "    ${CYAN}ubot status${NC}    - Check configuration"
    echo ""
    echo -e "  ${BOLD}Configuration:${NC} ~/.ubot/config.json"
    echo ""
    echo -e "${MAGENTA}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${MAGENTA}  Shipped to you by ${BOLD}Borkiss${NC}"
    echo -e "${MAGENTA}  https://github.com/lubluniky${NC}"
    echo -e "${MAGENTA}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
}

# Main installation flow
main() {
    print_banner

    detect_os
    check_requirements
    install_git
    install_docker
    setup_repository
    build_image
    setup_config
    create_service
    create_scripts

    print_complete
}

# Run main
main "$@"
