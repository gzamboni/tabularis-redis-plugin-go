#!/bin/bash
# Release script for tabularis-redis-plugin-go
# Usage: ./scripts/release.sh [patch|minor|major]

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Functions
error() {
    echo -e "${RED}Error: $1${NC}" >&2
    exit 1
}

info() {
    echo -e "${BLUE}$1${NC}"
}

success() {
    echo -e "${GREEN}$1${NC}"
}

warning() {
    echo -e "${YELLOW}$1${NC}"
}

# Check if git is clean
check_git_clean() {
    if [[ -n $(git status -s) ]]; then
        error "Working directory is not clean. Commit or stash changes first."
    fi
}

# Get current version from git tags
get_current_version() {
    local version=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
    echo "$version"
}

# Parse version components
parse_version() {
    local version=$1
    # Remove 'v' prefix if present
    version=${version#v}
    
    IFS='.' read -r -a parts <<< "$version"
    MAJOR="${parts[0]:-0}"
    MINOR="${parts[1]:-0}"
    PATCH="${parts[2]:-0}"
}

# Increment version
increment_version() {
    local bump_type=$1
    local current_version=$(get_current_version)
    
    parse_version "$current_version"
    
    case $bump_type in
        patch)
            PATCH=$((PATCH + 1))
            ;;
        minor)
            MINOR=$((MINOR + 1))
            PATCH=0
            ;;
        major)
            MAJOR=$((MAJOR + 1))
            MINOR=0
            PATCH=0
            ;;
        *)
            error "Invalid bump type: $bump_type. Use patch, minor, or major."
            ;;
    esac
    
    echo "v${MAJOR}.${MINOR}.${PATCH}"
}

# Update manifest.json version
update_manifest() {
    local new_version=$1
    # Remove 'v' prefix for manifest
    local version_no_v=${new_version#v}
    
    info "Updating manifest.json to version $version_no_v..."
    
    # Use sed to update version in manifest.json
    if [[ "$OSTYPE" == "darwin"* ]]; then
        # macOS
        sed -i '' "s/\"version\": \".*\"/\"version\": \"$version_no_v\"/" manifest.json
    else
        # Linux
        sed -i "s/\"version\": \".*\"/\"version\": \"$version_no_v\"/" manifest.json
    fi
    
    success "✓ manifest.json updated"
}

# Create git tag
create_tag() {
    local new_version=$1
    local current_version=$(get_current_version)
    
    info "Creating git tag $new_version..."
    
    # Commit manifest.json change
    git add manifest.json
    git commit -m "chore: bump version to $new_version"
    
    # Create annotated tag
    git tag -a "$new_version" -m "Release $new_version"
    
    success "✓ Tag $new_version created"
    info ""
    info "Previous version: $current_version"
    info "New version:      $new_version"
    info ""
    warning "To push the tag and trigger release workflow, run:"
    warning "  git push origin main"
    warning "  git push origin $new_version"
    info ""
    warning "Or use: make tag-push"
}

# Main script
main() {
    local bump_type=${1:-patch}
    
    info "=== Tabularis Redis Plugin Release Script ==="
    info ""
    
    # Validate bump type
    if [[ ! "$bump_type" =~ ^(patch|minor|major)$ ]]; then
        error "Invalid bump type: $bump_type. Use patch, minor, or major."
    fi
    
    # Check git status
    check_git_clean
    
    # Get current and new versions
    local current_version=$(get_current_version)
    local new_version=$(increment_version "$bump_type")
    
    info "Current version: $current_version"
    info "New version:     $new_version"
    info "Bump type:       $bump_type"
    info ""
    
    # Confirm
    read -p "Proceed with release? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        warning "Release cancelled."
        exit 0
    fi
    
    # Update manifest
    update_manifest "$new_version"
    
    # Create tag
    create_tag "$new_version"
    
    success ""
    success "=== Release $new_version ready! ==="
}

# Run main
main "$@"
