#!/bin/bash

# Color definitions for terminal output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
WHITE='\033[1;37m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${NC} $1"
}

log_success() {
    echo -e "${GREEN}${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
    echo -e "${NC} $1"
}

log_config() {
    echo -e "${CYAN}[CONFIG]${NC} $1"
}

# Default values
service_name="komari-agent"
target_dir="/opt/komari"
install_source=""
install_version="" # the panel's one-click command supplies the pinned release version
install_sha256="" # caller-pinned digest for custom download sources
 

# Detect OS
os_type=$(uname -s)
case $os_type in
    Darwin)
        os_name="darwin"
        target_dir="/usr/local/komari"  # Use /usr/local on macOS
        # Check if we can write to /usr/local, fallback to user directory
        if [ ! -w "/usr/local" ] && [ "$EUID" -ne 0 ]; then
            target_dir="$HOME/.komari"
            log_info "No write permission to /usr/local, using user directory: $target_dir"
        fi
        ;;
    Linux)
        os_name="linux"
        ;;
    FreeBSD)
        os_name="freebsd"
        ;;
    MINGW*|MSYS*|CYGWIN*)
        os_name="windows"
        target_dir="/c/komari"  # Use C:\komari on Windows
        ;;
    *)
        log_error "Unsupported operating system: $os_type"
        exit 1
        ;;
esac
[[ $os_name == linux ]] || { log_error 'This installer supports only Linux systemd or fresh OpenRC; use a native installer on this OS'; exit 1; }

# Parse install-specific arguments
komari_args=()
while [[ $# -gt 0 ]]; do
    case $1 in
        --install-dir|--install-service-name|--install-source|--install-ghproxy|--install-version|--install-sha256)
            if [[ $# -lt 2 || -z $2 || $2 == --install* ]]; then
                log_error "$1 requires a value"
                exit 1
            fi
            ;;
    esac
    case $1 in
        --install-dir)
            target_dir="$2"
            shift 2
            ;;
        --install-service-name)
            service_name="$2"
            shift 2
            ;;
        --install-source)
            install_source="${2%/}"
            shift 2
            ;;
        --install-ghproxy)
            # Kept for compatibility with old copied commands; no longer used.
            shift 2
            ;;
        --install-version)
            install_version="$2"
            shift 2
            ;;
        --install-sha256)
            install_sha256="$2"
            shift 2
            ;;
        --install*)
            log_warning "Unknown install parameter: $1"
            shift
            ;;
        *)
            # Non-install arguments go to the Agent without splitting or evaluation.
            komari_args+=("$1")
            shift
            ;;
    esac
done

extract_runtime_credentials() {
    local i=0 arg key value
    local -a remaining=()
    agent_token="" agent_endpoint="" agent_cf_secret=""
    while (( i < ${#komari_args[@]} )); do
        arg=${komari_args[i]}
        case $arg in
            -t|--token|-token|-e|--endpoint|-endpoint|--cf-access-client-secret|-cf-access-client-secret)
                if (( i + 1 >= ${#komari_args[@]} )) || [[ -z ${komari_args[i+1]} ]]; then
                    log_error "Missing Agent credential/endpoint value"
                    return 1
                fi
                key=$arg value=${komari_args[i+1]}
                ((i+=2)) ;;
            --token=*|-token=*|-t=*|-t?*|--endpoint=*|-endpoint=*|-e=*|-e?*|--cf-access-client-secret=*|-cf-access-client-secret=*)
                key=${arg%%=*} value=${arg#*=}
                if [[ $arg == -t?* && $arg != -t=* && $arg != -token=* ]]; then key=-t; value=${arg:2}; fi
                if [[ $arg == -e?* && $arg != -e=* ]]; then key=-e; value=${arg:2}; fi
                if [[ -z $value ]]; then log_error "Empty Agent credential/endpoint"; return 1; fi
                ((i+=1)) ;;
            --config*|-config*|--token*|-token*|--endpoint*|-endpoint*|--cf-access-client-secret*|-cf-access-client-secret*|--auto-discovery*|-auto-discovery*)
                log_error "Unsupported Agent config/credential syntax; use -t/--token and -e/--endpoint"
                return 1 ;;
            *) remaining+=("$arg"); ((i+=1)); continue ;;
        esac
        case $key in
            -t|--token|-token) agent_token=$value ;;
            -e|--endpoint|-endpoint) agent_endpoint=$value ;;
            --cf-access-client-secret|-cf-access-client-secret) agent_cf_secret=$value ;;
        esac
    done
    komari_args=("${remaining[@]}")
    if [[ $agent_endpoint == *://*@* ]]; then
        log_error "Agent endpoint URL userinfo is not allowed"
        return 1
    fi
}
extract_runtime_credentials || exit 1

validate_service_name() {
    [[ $1 =~ ^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$ && $1 != . && $1 != .. ]]
}
if ! validate_service_name "$service_name"; then
    log_error "Invalid --install-service-name (letters, digits, dot, underscore, hyphen only)."
    exit 1
fi
for value in "$target_dir" "$agent_token" "$agent_endpoint" "$agent_cf_secret" "${komari_args[@]}"; do
    if [[ $value == *[[:cntrl:]]* ]]; then
        log_error "Control characters are not allowed in install paths or Agent arguments."
        exit 1
    fi
done
if [[ $target_dir != /* ]]; then
    log_error "--install-dir must be an absolute path"
    exit 1
fi
# Validate before any logs or network requests; URL userinfo is often a secret.
if [[ $install_source == *[[:cntrl:]]* || $install_source == *://*@* || $install_source == *[\?\#\\]* ]]; then
    log_error "Invalid --install-source: URL userinfo and query/fragment are not allowed"
    exit 1
fi
if [[ -n $agent_endpoint && ( $agent_endpoint != https://* || $agent_endpoint == *[[:cntrl:]]* || $agent_endpoint == *://*@* ) ]]; then
    log_error "Invalid Agent endpoint (HTTPS without userinfo required)"
    exit 1
fi

# Single-quote one shell word for service scripts (including embedded apostrophes).
shell_quote() {
    local value=$1 quoted="'" char i
    for ((i=0; i<${#value}; i++)); do
        char=${value:i:1}
        if [[ $char == "'" ]]; then quoted+="'\\''"; else quoted+="$char"; fi
    done
    printf "%s'" "$quoted"
}
# systemd has its own escaping rules; $$ prevents environment expansion.
systemd_quote() {
    local value=${1//\\/\\\\}
    value=${value//\"/\\\"}
    value=${value//\$/\$\$}
    value=${value//%/%%}
    printf '"%s"' "$value"
}
# WorkingDirectory= is a single path, not an ExecStart argument list. Quotes
# become literal path characters there and make an absolute path invalid.
systemd_workdir() {
    local value=${1//\\/\\\\}
    value=${value//%/%%}
    printf '%s' "$value"
}
xml_escape() {
    local value=${1//&/\&amp;}
    value=${value//</\&lt;}
    value=${value//>/\&gt;}
    value=${value//\"/\&quot;}
    value=${value//\'/\&apos;}
    printf '%s' "$value"
}
nix_escape() {
    local value=${1//\\/\\\\}
    value=${value//\"/\\\"}
    value=${value//\$\{/\\\$\{}
    printf '%s' "$value"
}
json_escape() {
    local value=${1//\\/\\\\}
    value=${value//\"/\\\"}
    printf '%s' "$value"
}

komari_agent_path="${target_dir}/agent"

# macOS doesn't always require sudo for everything
if [ "$os_name" = "darwin" ] && command -v brew >/dev/null 2>&1; then
    # On macOS with Homebrew, we can run without root for dependencies
    require_root_for_deps=false
else
    require_root_for_deps=true
fi

if [ "$EUID" -ne 0 ] && [ "$require_root_for_deps" = true ]; then
    log_error "Please run as root"
    exit 1
fi

echo -e "${WHITE}===========================================${NC}"
echo -e "${WHITE}    Komari Agent Installation Script     ${NC}"
echo -e "${WHITE}===========================================${NC}"
echo ""
log_config "Installation configuration:"
log_config "  Service name: ${GREEN}$service_name${NC}"
log_config "  Install directory: ${GREEN}$target_dir${NC}"
log_config "  Agent source: ${GREEN}${install_source:-"(missing)"}${NC}"
log_config "  Agent arguments: [redacted] (${#komari_args[@]} arguments)"
if [ -n "$install_version" ]; then
    log_config "  Specified agent version: ${GREEN}$install_version${NC}"
else
    log_config "  Agent version: ${GREEN}panel-managed${NC}"
fi
echo ""


install_dependencies() {
    log_step "Checking and installing dependencies..."

    local deps="curl"
    local missing_deps=""
    for cmd in $deps; do
        if ! command -v $cmd >/dev/null 2>&1; then
            missing_deps="$missing_deps $cmd"
        fi
    done

    if [ -n "$missing_deps" ]; then
        # Check package manager and install dependencies
        if command -v apt >/dev/null 2>&1; then
            log_info "Using apt to install dependencies..."
            apt update
            apt install -y $missing_deps
        elif command -v yum >/dev/null 2>&1; then
            log_info "Using yum to install dependencies..."
            yum install -y $missing_deps
        elif command -v apk >/dev/null 2>&1; then
            log_info "Using apk to install dependencies..."
            apk add $missing_deps
        elif command -v brew >/dev/null 2>&1; then
            log_info "Using Homebrew to install dependencies..."
            brew install $missing_deps
        else
            log_error "No supported package manager found (apt/yum/apk/brew)"
            exit 1
        fi
        
        # Verify installation
        for cmd in $missing_deps; do
            if ! command -v $cmd >/dev/null 2>&1; then
                log_error "Failed to install $cmd"
                exit 1
            fi
        done
        log_success "Dependencies installed successfully"
    else
        log_success "Dependencies already satisfied"
    fi
}

 
# Validate release inputs before installing dependencies.

 

# Architecture detection with platform-specific support
arch=$(uname -m)
case $arch in
    x86_64)
        arch="amd64"
        ;;
    aarch64|arm64)
        arch="arm64"
        ;;
    loongarch64|loong64)
        arch="loong64"
        ;;
    i386|i686)
        # x86 (32-bit) support
        case $os_name in
            freebsd|linux|windows)
                arch="386"
                ;;
            *)
                log_error "32-bit x86 architecture not supported on $os_name"
                exit 1
                ;;
        esac
        ;;
    armv7*|armv6*)
        # ARM 32-bit support
        case $os_name in
            freebsd|linux)
                arch="arm"
                ;;
            *)
                log_error "32-bit ARM architecture not supported on $os_name"
                exit 1
                ;;
        esac
        ;;
    *)
        log_error "Unsupported architecture: $arch on $os_name"
        exit 1
        ;;
esac
log_info "Detected OS: ${GREEN}$os_name${NC}, Architecture: ${GREEN}$arch${NC}"

if [ -n "$install_version" ]; then
    log_info "Installing pinned version: ${GREEN}$install_version${NC}"
else
    log_info "No version specified; using the explicitly configured Agent source."
fi

# Construct the Agent download URL from the explicitly configured release source.
file_name="komari-agent-${os_name}-${arch}"
if [ -z "$install_source" ]; then
    log_error "Missing --install-source (expected: a pinned GitHub release URL)"
    exit 1
fi
# The standard release keeps its existing HTTPS checksum manifest workflow.
# A mirror or custom origin needs an independently supplied, fixed digest: a
# checksum downloaded from that same untrusted origin proves nothing.
if [[ ! $install_source =~ ^https://[^/]+(/.*)?$ ]]; then
    log_error "--install-source must be an HTTPS URL"
    exit 1
fi
if [ -n "$install_sha256" ] && [[ ! $install_sha256 =~ ^[[:xdigit:]]{64}$ ]]; then
    log_error "Invalid --install-sha256 (expected 64 hexadecimal characters)"
    exit 1
fi
if [[ ! $install_source =~ ^https://github[.]com/3rnn/komari-lite/releases/download/v[^/?#]+$ ]] && [ -z "$install_sha256" ]; then
    log_error "Custom --install-source requires --install-sha256 from a trusted, independent source"
    exit 1
fi
validate_release_version() {
    [[ -z $install_version ]] && return 0
    if [[ ! $install_version =~ ^[0-9]+([.][0-9]+){1,3}([-+][A-Za-z0-9.-]+)?$ ||
          $install_source != "https://github.com/3rnn/komari-lite/releases/download/v${install_version}" ]]; then
        log_error "Invalid --install-version: requested version must match the official release URL exactly"
        return 1
    fi
}
validate_release_version || exit 1

# Dependency installation happens only after platform detection/preflight.

download_url="${install_source}/${file_name}"

# Refuse symlinks, dot components, foreign ownership and group/world-writable
# ancestors. Create missing components privately, checking each after creation.
validate_install_directory() {
    local path=$1 current=/ part owner mode uid
    [[ $path == /* && $path != / && $path != *//* ]] || { log_error 'Invalid installation directory'; return 1; }
    uid=$EUID
    local -a parts
    IFS=/ read -r -a parts <<< "${path#/}"
    for part in "${parts[@]}"; do
        [[ -n $part && $part != . && $part != .. ]] || { log_error 'Invalid installation directory component'; return 1; }
        current="${current%/}/$part"
        if [[ -L $current ]]; then log_error 'Symlink in installation directory'; return 1; fi
        if [[ ! -e $current ]]; then
            (umask 077; mkdir "$current") || return 1
        fi
        [[ -d $current ]] || { log_error 'Non-directory in installation path'; return 1; }
        if stat -c '%u %a' "$current" >/dev/null 2>&1; then
            read -r owner mode < <(stat -c '%u %a' "$current")
        else
            read -r owner mode < <(stat -f '%u %Lp' "$current") || return 1
        fi
        if [[ $owner != 0 && $owner != "$uid" ]] || (( (8#$mode & 0022) != 0 )); then
            log_error 'Unsafe installation directory ownership or mode'; return 1
        fi
    done
}
# Detect init system and configure service
log_step "Configuring system service..."

# Function to detect actual init system
detect_init_system() {
    # Check if running on NixOS (special case)
    if [ -f /etc/NIXOS ]; then
        echo "nixos"
        return
    fi
    
    # Alpine Linux MUST be checked first
    # Alpine always uses OpenRC, even in containers where PID 1 might be different
    if [ -f /etc/alpine-release ]; then
        if command -v rc-service >/dev/null 2>&1 || [ -f /sbin/openrc-run ]; then
            echo "openrc"
            return
        fi
    fi
    
    # Get PID 1 process for other detection
    local pid1_process=$(ps -p 1 -o comm= 2>/dev/null | tr -d ' ')
    
    # If PID 1 is systemd, use systemd
    if [ "$pid1_process" = "systemd" ] || [ -d /run/systemd/system ]; then
        if command -v systemctl >/dev/null 2>&1; then
            # Additional verification that systemd is actually functioning
            if systemctl list-units >/dev/null 2>&1; then
                echo "systemd"
                return
            fi
        fi
    fi
    
    # Check for Gentoo OpenRC (PID 1 is openrc-init)
    if [ "$pid1_process" = "openrc-init" ]; then
        if command -v rc-service >/dev/null 2>&1; then
            echo "openrc"
            return
        fi
    fi
    
    # Check for other OpenRC systems (not Alpine, already handled)
    # Some systems use traditional init with OpenRC
    if [ "$pid1_process" = "init" ] && [ ! -f /etc/alpine-release ]; then
        # Check if OpenRC is actually managing services
        if [ -d /run/openrc ] && command -v rc-service >/dev/null 2>&1; then
            echo "openrc"
            return
        fi
        # Check for OpenRC files
        if [ -f /sbin/openrc ] && command -v rc-service >/dev/null 2>&1; then
            echo "openrc"
            return
        fi
    fi
    
    # Check for OpenWrt's procd
    if command -v uci >/dev/null 2>&1 && [ -f /etc/rc.common ]; then
        echo "procd"
        return
    fi
    
    # Check for macOS launchd
    if [ "$os_name" = "darwin" ] && command -v launchctl >/dev/null 2>&1; then
        echo "launchd"
        return
    fi
    
    # Fallback: if systemctl exists and appears functional, assume systemd
    if command -v systemctl >/dev/null 2>&1; then
        if systemctl list-units >/dev/null 2>&1; then
            echo "systemd"
            return
        fi
    fi
    
    # Last resort: check for OpenRC without other indicators
    if command -v rc-service >/dev/null 2>&1 && [ -d /etc/init.d ]; then
        echo "openrc"
        return
    fi

    # check for Upstart (CentOS 6)
    if command -v initctl >/dev/null 2>&1 && [ -d /etc/init ]; then
        echo "upstart"
        return
    fi
    
    echo "unknown"
}

init_system=$(detect_init_system)
log_info "Detected init system: ${GREEN}$init_system${NC}"
case $init_system in
    systemd|openrc) ;;
    *) log_error "Unsupported init system: $init_system (only systemd and fresh OpenRC are supported)"; exit 1 ;;
esac

if [[ $init_system == openrc && ! $target_dir =~ ^/[A-Za-z0-9_./-]+$ ]]; then
    log_error "OpenRC requires an install path without shell metacharacters or spaces"
    exit 1
fi


verify_systemd_execstart() {
    local effective=$1 actual
    # systemctl show reports the resolved executable, after unit/drop-in parsing.
    # Only accept one unambiguous, unescaped path; comments cannot supply it.
    [[ $effective == \{\ path=*' ; '* && $effective == *' }' &&
       $effective != *$'\n'* && $effective != *'} {'* ]] || return 1
    actual=${effective#\{ path=}
    actual=${actual%% ; *}
    [[ -n $actual && $actual == "$komari_agent_path" ]] || {
        log_error 'Existing systemd ExecStart targets a different executable'; return 1;
    }
}

verify_existing_service() {
    local file='' fragment='' state='' owner='' mode='' effective=''
    case $init_system in
        systemd)
            file="/etc/systemd/system/${service_name}.service"
            state=$(systemctl show "${service_name}.service" -p LoadState --value) || return 1
            fragment=$(systemctl show "${service_name}.service" -p FragmentPath --value) || return 1
            if [[ $state != not-found && $fragment != "$file" ]]; then
                log_error 'Existing systemd service has foreign provenance'; return 1
            fi
            if [[ $state != not-found ]]; then
                local dropins
                dropins=$(systemctl show "${service_name}.service" -p DropInPaths --value) || return 1
                [[ -z $dropins ]] || { log_error 'Existing systemd service has unverified drop-ins'; return 1; }
            fi ;;
        openrc)
            file="/etc/init.d/${service_name}"
            [[ ! -e $file && ! -L $file ]] || { log_error 'OpenRC upgrades require manual migration; existing service was not changed'; return 1; } ;;
        *) log_error 'Unsupported init system'; return 1 ;;
    esac
    if [[ -e $file || -L $file ]]; then
        [[ -f $file && ! -L $file ]] || { log_error 'Unsafe service file'; return 1; }
        if stat -c '%u %a' "$file" >/dev/null 2>&1; then
            read -r owner mode < <(stat -c '%u %a' "$file")
        else
            read -r owner mode < <(stat -f '%u %Lp' "$file") || return 1
        fi
        [[ $owner == 0 || $owner == "$EUID" ]] && (( (8#$mode & 0022) == 0 )) || {
            log_error 'Unsafe service file ownership or mode'; return 1;
        }
        grep -Fxq '# komari-agent installer' "$file" || {
            log_error 'Existing service lacks installer provenance'; return 1;
        }
        case $init_system in
            systemd)
                [[ $state != not-found ]] || return 1
                effective=$(systemctl show "${service_name}.service" -p ExecStart --value) || return 1
                if [[ $effective =~ (^|[[:space:]])(--?token|--?cf-access-client-secret)(=|[[:space:]]|$) ||
                      $effective =~ (^|[[:space:]])-t(=|[[:space:]]|[^[:space:]]) ]]; then
                    log_error 'Existing service contains credential arguments; manual migration required'
                    return 1
                fi
                verify_systemd_execstart "$effective" || return 1 ;;
            *) log_error 'Cannot verify existing service executable'; return 1 ;;
        esac
    elif [[ $init_system == systemd && $state != not-found ]]; then
        log_error 'Existing service has no verified installer file'; return 1
    elif [[ $init_system == systemd && ( -e $komari_agent_path || -e $target_dir/agent-config.json ) ]]; then
        log_error 'Existing Agent artifacts have no verified installer service'; return 1
    fi
    for file in "$komari_agent_path" "$target_dir/agent-config.json" "$target_dir/agent-service.sh"; do
        [[ ! -L $file ]] || { log_error 'Symlink at existing Agent artifact'; return 1; }
    done
}
verify_existing_service || exit 1
if [[ $init_system == openrc && ( -e $komari_agent_path || -e $target_dir/agent-config.json || -e $target_dir/agent-service.sh ) ]]; then
    log_error 'OpenRC artifacts already exist; manual migration required before installation'
    exit 1
fi
install_dependencies
log_step "Creating installation directory: ${GREEN}$target_dir${NC}"
validate_install_directory "$target_dir" || exit 1

verify_started_service() {
    local state domain
    sleep 2
    case $1 in
        systemd) systemctl is-active --quiet "${service_name}.service" || return 1 ;;
        openrc) rc-service "$service_name" status >/dev/null 2>&1 || return 1 ;;

        *) return 1 ;;
    esac
}

staging_dir=$(mktemp -d "${target_dir}/.agent-download.XXXXXXXX") || exit 1
trap 'if [[ ${transaction_pending:-no} == yes ]]; then rollback_install; fi; if [[ ${retain_staging:-no} != yes ]]; then rm -rf "$staging_dir"; fi; [[ -z ${config_tmp:-} ]] || rm -f "$config_tmp"' EXIT
trap 'exit 1' INT TERM
staged_agent="${staging_dir}/${file_name}"

# Download and check the new binary before removing a working installation.
log_step "Downloading $file_name from the pinned release..."
log_info "URL: ${CYAN}$download_url${NC}"
if ! curl --fail --show-error -L -o "$staged_agent" "$download_url"; then
    log_error "Download failed"
    exit 1
fi

if [ -n "$install_sha256" ]; then
    expected=${install_sha256,,}
else
    checksum_file="${staging_dir}/SHA256SUMS.txt"
    if ! curl --fail --show-error -L -o "$checksum_file" "${install_source}/SHA256SUMS.txt"; then
        log_error "Unable to download release checksums"
        exit 1
    fi
    expected=$(awk -v name="$file_name" '$2 == name && $1 ~ /^[[:xdigit:]]+$/ && length($1) == 64 { print tolower($1) }' "$checksum_file")
    if [ -z "$expected" ] || [ "${#expected}" -ne 64 ]; then
        log_error "Release checksum missing or ambiguous for $file_name"
        exit 1
    fi
fi
if command -v sha256sum >/dev/null 2>&1; then
    actual=$(sha256sum "$staged_agent" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
    actual=$(shasum -a 256 "$staged_agent" | awk '{print $1}')
elif command -v sha256 >/dev/null 2>&1; then
    actual=$(sha256 -q "$staged_agent")
else
    log_error "No SHA-256 tool available to verify the release binary"
    exit 1
fi
if [ "$actual" != "$expected" ]; then
    log_error "Agent checksum mismatch for $file_name"
    exit 1
fi
log_success "Verified $file_name against SHA-256 checksum"

# Set executable permissions
chmod +x "$staged_agent"
# Only the private JSON config carries runtime credentials, never service argv.
umask 077
config_path="${target_dir}/agent-config.json"
config_tmp=$(mktemp "${target_dir}/.agent-config.XXXXXXXX") || exit 1
printf '{"token":"%s","endpoint":"%s","cf_access_client_secret":"%s"}\n' \
    "$(json_escape "$agent_token")" "$(json_escape "$agent_endpoint")" "$(json_escape "$agent_cf_secret")" > "$config_tmp"
chmod 600 "$config_tmp" || exit 1
komari_args+=("--config" "$config_path")
# Serialize each Agent argument for the destination format before writing service files.
shell_command=$(shell_quote "$komari_agent_path")
systemd_command=$(systemd_quote "$komari_agent_path")
shell_args=""
for arg in "${komari_args[@]}"; do
    shell_args+=" $(shell_quote "$arg")"
    systemd_command+=" $(systemd_quote "$arg")"
done
# Stage service definitions; no working artifact is replaced until rollback
# copies and service state have been captured.
if [ "$init_system" = "openrc" ]; then
    # OpenRC service configuration
    log_info "Using OpenRC for service management"
    service_file="/etc/init.d/${service_name}"
    # OpenRC may eval command_args. Put caller arguments only in a shell-quoted
    # private wrapper; its path is restricted to safe literal characters.
    wrapper="${target_dir}/agent-service.sh"
    staged_wrapper="${staging_dir}/agent-service.sh"
    staged_service="${staging_dir}/service"
    cat > "$staged_wrapper" << EOF
#!/bin/sh
exec ${shell_command}${shell_args}
EOF
    chmod 700 "$staged_wrapper"
    cat > "$staged_service" << EOF
#!/sbin/openrc-run
# komari-agent installer

name="Komari Agent Service"
description="Komari monitoring agent"
command=$(shell_quote "$wrapper")
command_args=""
command_user="root"
directory=$(shell_quote "$target_dir")
pidfile="/run/${service_name}.pid"
retry="SIGTERM/30"
supervisor=supervise-daemon

depend() {
    need net
    after network
}
EOF

    chmod 700 "$staged_service" || exit 1
elif [ "$init_system" = "systemd" ]; then
    # Systemd service configuration
    log_info "Using systemd for service management"
    service_file="/etc/systemd/system/${service_name}.service"
    staged_service="${staging_dir}/service"
    cat > "$staged_service" << EOF
[Unit]
# komari-agent installer
Description=Komari Agent Service
After=network.target

[Service]
Type=simple
ExecStart=${systemd_command}
WorkingDirectory=$(systemd_workdir "$target_dir")
Restart=always
User=root

[Install]
WantedBy=multi-user.target
EOF

    chmod 600 "$staged_service" || exit 1
fi

# Backups remain private until the replacement is confirmed running. If
# rollback itself fails, retain them for manual recovery.
rollback_install() {
    local path name failed=0
    transaction_pending=no
    if [[ $init_system == systemd ]]; then
        systemctl stop "${service_name}.service" >/dev/null 2>&1 || :
    else
        rc-service "$service_name" stop >/dev/null 2>&1 || :
    fi
    for name in agent agent-config.json service agent-service.sh; do
        case $name in
            agent) path=$komari_agent_path ;;
            agent-config.json) path=$config_path ;;
            service) path=$service_file ;;
            agent-service.sh) [[ $init_system == openrc ]] || continue; path=$wrapper ;;
        esac
        if [[ -f $staging_dir/backup-$name ]]; then
            cp -p "$staging_dir/backup-$name" "$path" || failed=1
        else
            rm -f "$path" || failed=1
        fi
    done
    if [[ $init_system == systemd ]]; then
        systemctl daemon-reload || failed=1
        if [[ $previous_enabled == yes ]]; then
            systemctl enable "${service_name}.service" || failed=1
        else
            systemctl disable "${service_name}.service" || failed=1
        fi
        if [[ $previous_running == yes ]]; then
            systemctl start "${service_name}.service" || failed=1
            systemctl is-active --quiet "${service_name}.service" || failed=1
        fi
    else
        rc-update del "$service_name" default >/dev/null 2>&1 || :
    fi
    if (( failed )); then
        retain_staging=yes
        log_error "Rollback incomplete; recovery copies retained in $staging_dir"
        return 1
    fi
    log_error 'Installation failed; previous artifacts and service state restored'
}

commit_install() {
    local path name
    previous_running=no previous_enabled=no
    if [[ $init_system == systemd ]]; then
        if systemctl is-active --quiet "${service_name}.service"; then previous_running=yes; fi
        if systemctl is-enabled --quiet "${service_name}.service"; then previous_enabled=yes; fi
    fi
    for name in agent agent-config.json service agent-service.sh; do
        case $name in
            agent) path=$komari_agent_path ;;
            agent-config.json) path=$config_path ;;
            service) path=$service_file ;;
            agent-service.sh) [[ $init_system == openrc ]] || continue; path=$wrapper ;;
        esac
        if [[ -e $path ]]; then
            [[ -f $path && ! -L $path ]] || return 1
            cp -p "$path" "$staging_dir/backup-$name" || return 1
        fi
    done
    # Keep the old unit enabled throughout the upgrade.
    transaction_pending=yes
    if [[ $previous_running == yes ]]; then
        systemctl stop "${service_name}.service" || { rollback_install; return 1; }
    fi
    if ! mv -f "$staged_agent" "$komari_agent_path" ||
       ! mv -f "$config_tmp" "$config_path" ||
       { [[ $init_system == openrc ]] && ! mv -f "$staged_wrapper" "$wrapper"; } ||
       ! mv -f "$staged_service" "$service_file"; then
        rollback_install
        return 1
    fi
    if [[ $init_system == systemd ]]; then
        systemctl daemon-reload &&
            { [[ $previous_enabled == yes ]] || systemctl enable "${service_name}.service"; } &&
            systemctl start "${service_name}.service" &&
            verify_started_service systemd || { rollback_install; return 1; }
    else
        rc-update add "$service_name" default &&
            rc-service "$service_name" start &&
            verify_started_service openrc || { rollback_install; return 1; }
    fi
    transaction_pending=no
    return 0
}

commit_install || exit 1
rm -rf "$staging_dir" || exit 1
trap - EXIT
log_success "Komari-agent installation completed; service is running"
