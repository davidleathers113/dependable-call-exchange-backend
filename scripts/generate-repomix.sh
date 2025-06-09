#!/bin/bash

# generate-repomix.sh
# Generates both compressed and full repomix outputs for the DependableCallExchangeBackEnd project
# 
# Usage:
#   ./scripts/generate-repomix.sh           # Generate both versions
#   ./scripts/generate-repomix.sh full      # Generate only full version
#   ./scripts/generate-repomix.sh compress  # Generate only compressed version
#   ./scripts/generate-repomix.sh clean     # Remove all repomix outputs

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Project info
PROJECT_NAME="DependableCallExchangeBackEnd"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")

# Output file paths
COMPRESSED_OUTPUT="repomix-output-compressed.md"
FULL_OUTPUT="repomix-output-full.md"
ARCHIVE_DIR="repomix-archive"

print_usage() {
    echo -e "${BLUE}Usage:${NC}"
    echo "  $0                # Generate both compressed and full versions"
    echo "  $0 full          # Generate only full version"  
    echo "  $0 compress      # Generate only compressed version"
    echo "  $0 clean         # Remove all repomix outputs"
    echo "  $0 archive       # Archive current outputs with timestamp"
    echo ""
    echo -e "${BLUE}Output files:${NC}"
    echo "  ${COMPRESSED_OUTPUT} - Compressed version (structure-focused)"
    echo "  ${FULL_OUTPUT} - Full version (complete implementation)"
    echo ""
}

archive_outputs() {
    if [[ -f "$COMPRESSED_OUTPUT" ]] || [[ -f "$FULL_OUTPUT" ]]; then
        echo -e "${YELLOW}📦 Archiving existing outputs...${NC}"
        mkdir -p "$ARCHIVE_DIR"
        
        if [[ -f "$COMPRESSED_OUTPUT" ]]; then
            mv "$COMPRESSED_OUTPUT" "$ARCHIVE_DIR/repomix-compressed-${TIMESTAMP}.md"
            echo -e "   Archived: ${ARCHIVE_DIR}/repomix-compressed-${TIMESTAMP}.md"
        fi
        
        if [[ -f "$FULL_OUTPUT" ]]; then
            mv "$FULL_OUTPUT" "$ARCHIVE_DIR/repomix-full-${TIMESTAMP}.md"
            echo -e "   Archived: ${ARCHIVE_DIR}/repomix-full-${TIMESTAMP}.md"
        fi
        echo ""
    fi
}

generate_compressed() {
    echo -e "${BLUE}🗜️  Generating compressed repomix (structure-focused)...${NC}"
    echo -e "   ${YELLOW}Using tree-sitter to extract signatures and interfaces${NC}"
    
    repomix \
        --output "$COMPRESSED_OUTPUT" \
        --style markdown \
        --compress \
        --output-show-line-numbers \
        --header-text "DependableCallExchangeBackEnd - COMPRESSED VERSION (Structure-Focused)
Generated: $(date)
Contains: Function signatures, interfaces, struct definitions
Missing: Implementation details, business logic, comments
Best for: Architecture analysis, pattern review, high-level understanding"
        
    if [[ -f "$COMPRESSED_OUTPUT" ]]; then
        local file_size=$(du -h "$COMPRESSED_OUTPUT" | cut -f1)
        local line_count=$(wc -l < "$COMPRESSED_OUTPUT")
        echo -e "${GREEN}✅ Compressed version generated: ${COMPRESSED_OUTPUT}${NC}"
        echo -e "   Size: ${file_size}, Lines: ${line_count}"
        echo -e "   ${YELLOW}Ideal for: Architecture review, refactoring planning, pattern analysis${NC}"
    else
        echo -e "${RED}❌ Failed to generate compressed version${NC}"
        exit 1
    fi
}

generate_full() {
    echo -e "${BLUE}📄 Generating full repomix (complete implementation)...${NC}"
    echo -e "   ${YELLOW}Including all code, comments, and implementation details${NC}"
    
    repomix \
        --output "$FULL_OUTPUT" \
        --style markdown \
        --output-show-line-numbers \
        --header-text "DependableCallExchangeBackEnd - FULL VERSION (Complete Implementation)
Generated: $(date)
Contains: Complete source code, business logic, comments, implementation details
Best for: Debugging, code generation, detailed analysis, implementation understanding"
        
    if [[ -f "$FULL_OUTPUT" ]]; then
        local file_size=$(du -h "$FULL_OUTPUT" | cut -f1)
        local line_count=$(wc -l < "$FULL_OUTPUT")
        echo -e "${GREEN}✅ Full version generated: ${FULL_OUTPUT}${NC}"
        echo -e "   Size: ${file_size}, Lines: ${line_count}"
        echo -e "   ${YELLOW}Ideal for: Debugging, code generation, detailed implementation analysis${NC}"
    else
        echo -e "${RED}❌ Failed to generate full version${NC}"
        exit 1
    fi
}

clean_outputs() {
    echo -e "${YELLOW}🧹 Cleaning repomix outputs...${NC}"
    
    local cleaned=false
    if [[ -f "$COMPRESSED_OUTPUT" ]]; then
        rm "$COMPRESSED_OUTPUT"
        echo -e "   Removed: $COMPRESSED_OUTPUT"
        cleaned=true
    fi
    
    if [[ -f "$FULL_OUTPUT" ]]; then
        rm "$FULL_OUTPUT" 
        echo -e "   Removed: $FULL_OUTPUT"
        cleaned=true
    fi
    
    if [[ "$cleaned" == "true" ]]; then
        echo -e "${GREEN}✅ Cleanup complete${NC}"
    else
        echo -e "${YELLOW}ℹ️  No outputs to clean${NC}"
    fi
}

show_summary() {
    echo ""
    echo -e "${BLUE}📊 Summary:${NC}"
    
    if [[ -f "$COMPRESSED_OUTPUT" ]]; then
        local comp_size=$(du -h "$COMPRESSED_OUTPUT" | cut -f1)
        echo -e "   ${GREEN}Compressed:${NC} $COMPRESSED_OUTPUT (${comp_size})"
    fi
    
    if [[ -f "$FULL_OUTPUT" ]]; then
        local full_size=$(du -h "$FULL_OUTPUT" | cut -f1)
        echo -e "   ${GREEN}Full:${NC} $FULL_OUTPUT (${full_size})"
    fi
    
    echo ""
    echo -e "${BLUE}💡 Usage Tips:${NC}"
    echo -e "   • Use ${YELLOW}compressed${NC} for architecture analysis and planning"
    echo -e "   • Use ${YELLOW}full${NC} for debugging and implementation details" 
    echo -e "   • Both files are .gitignored and safe to regenerate"
    echo ""
}

# Main execution
case "${1:-both}" in
    "compress"|"compressed")
        generate_compressed
        show_summary
        ;;
    "full")
        generate_full
        show_summary
        ;;
    "clean")
        clean_outputs
        ;;
    "archive")
        archive_outputs
        ;;
    "both"|"")
        echo -e "${BLUE}🚀 Generating both compressed and full repomix outputs for ${PROJECT_NAME}${NC}"
        echo ""
        generate_compressed
        echo ""
        generate_full
        show_summary
        ;;
    "help"|"-h"|"--help")
        print_usage
        ;;
    *)
        echo -e "${RED}❌ Unknown option: $1${NC}"
        echo ""
        print_usage
        exit 1
        ;;
esac