#!/bin/bash
# Benchmark runner script for go-reddit-api-wrapper
# This script runs benchmarks with optimal settings and supports comparison mode

set -euo pipefail

# Configuration
BENCHMARK_FLAGS="-benchmem -benchtime=1s -count=5"
OUTPUT_DIR="benchmarks/baselines"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
GO_VERSION=$(go version | awk '{print $3}')
OS_INFO=$(uname -s)
CPU_INFO=$(uname -m)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Help message
show_help() {
    cat << EOF
Usage: $(basename "$0") [OPTIONS]

Run benchmarks for the go-reddit-api-wrapper project

OPTIONS:
    -h, --help          Show this help message
    -a, --all           Run all benchmarks (default)
    -u, --unit          Run only unit benchmarks
    -i, --integration   Run only integration benchmarks
    -c, --comparative   Run only comparative benchmarks
    -s, --save          Save results to baselines directory
    -C, --compare FILE  Compare with previous baseline file using benchstat
    -v, --verbose       Show verbose output
    -t, --test PATTERN  Run benchmarks matching pattern (e.g., "BufferPool")

EXAMPLES:
    # Run all benchmarks and save results
    ./benchmarks/run.sh --save

    # Run only unit benchmarks
    ./benchmarks/run.sh --unit

    # Compare with previous run
    ./benchmarks/run.sh --save --compare benchmarks/baselines/baseline-20250101.txt

    # Run specific benchmark
    ./benchmarks/run.sh --test "BenchmarkBufferPool"

EOF
}

# Parse arguments
RUN_UNIT=false
RUN_INTEGRATION=false
RUN_COMPARATIVE=false
RUN_ALL=true
SAVE_RESULTS=false
COMPARE_FILE=""
VERBOSE=false
TEST_PATTERN=""

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -a|--all)
            RUN_ALL=true
            shift
            ;;
        -u|--unit)
            RUN_UNIT=true
            RUN_ALL=false
            shift
            ;;
        -i|--integration)
            RUN_INTEGRATION=true
            RUN_ALL=false
            shift
            ;;
        -c|--comparative)
            RUN_COMPARATIVE=true
            RUN_ALL=false
            shift
            ;;
        -s|--save)
            SAVE_RESULTS=true
            shift
            ;;
        -C|--compare)
            COMPARE_FILE="$2"
            shift 2
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -t|--test)
            TEST_PATTERN="$2"
            shift 2
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            show_help
            exit 1
            ;;
    esac
done

# Set run flags based on options
if [ "$RUN_ALL" = true ]; then
    RUN_UNIT=true
    RUN_INTEGRATION=true
    RUN_COMPARATIVE=true
fi

# Check if benchstat is available for comparisons
if [ -n "$COMPARE_FILE" ]; then
    if ! command -v benchstat &> /dev/null; then
        echo -e "${YELLOW}Warning: benchstat not found. Install with: go install golang.org/x/perf/cmd/benchstat@latest${NC}"
        echo "Comparison will be skipped."
        COMPARE_FILE=""
    elif [ ! -f "$COMPARE_FILE" ]; then
        echo -e "${RED}Error: Comparison file not found: $COMPARE_FILE${NC}"
        exit 1
    fi
fi

# Create output directory if saving
if [ "$SAVE_RESULTS" = true ]; then
    mkdir -p "$OUTPUT_DIR"
    OUTPUT_FILE="$OUTPUT_DIR/baseline-$TIMESTAMP.txt"
    echo -e "${GREEN}Results will be saved to: $OUTPUT_FILE${NC}"

    # Write metadata
    {
        echo "# Benchmark Results"
        echo "# Timestamp: $TIMESTAMP"
        echo "# Go Version: $GO_VERSION"
        echo "# OS: $OS_INFO"
        echo "# CPU: $CPU_INFO"
        echo ""
    } > "$OUTPUT_FILE"
fi

# Function to run benchmarks
run_benchmarks() {
    local package=$1
    local name=$2

    echo -e "${GREEN}Running $name benchmarks...${NC}"

    local cmd="go test $BENCHMARK_FLAGS"

    if [ -n "$TEST_PATTERN" ]; then
        cmd="$cmd -bench=$TEST_PATTERN"
    else
        cmd="$cmd -bench=."
    fi

    if [ "$VERBOSE" = true ]; then
        cmd="$cmd -v"
    fi

    cmd="$cmd $package"

    echo -e "${YELLOW}Command: $cmd${NC}"

    if [ "$SAVE_RESULTS" = true ]; then
        echo "# $name" >> "$OUTPUT_FILE"
        eval "$cmd" | tee -a "$OUTPUT_FILE"
        echo "" >> "$OUTPUT_FILE"
    else
        eval "$cmd"
    fi
}

echo -e "${GREEN}=====================================${NC}"
echo -e "${GREEN}  go-reddit-api-wrapper Benchmarks  ${NC}"
echo -e "${GREEN}=====================================${NC}"
echo ""

# Run unit benchmarks
if [ "$RUN_UNIT" = true ]; then
    echo -e "${GREEN}=== Unit Benchmarks ===${NC}"

    # HTTP Client benchmarks
    if [ -f "reddit/internal/client/client_bench_test.go" ]; then
        run_benchmarks "./reddit/internal/client" "HTTP Client"
    fi

    # Parser benchmarks
    if [ -f "reddit/internal/parse/parse_bench_test.go" ]; then
        run_benchmarks "./reddit/internal/parse" "Parser"
    fi

    # Auth benchmarks
    if [ -f "reddit/internal/auth/auth_bench_test.go" ]; then
        run_benchmarks "./reddit/internal/auth" "Authentication"
    fi

    # Validator benchmarks (already exists)
    run_benchmarks "./reddit/internal/validator" "Validator"
    run_benchmarks "./pkg/validation" "Validation"
fi

# Run integration benchmarks
if [ "$RUN_INTEGRATION" = true ]; then
    echo -e "${GREEN}=== Integration Benchmarks ===${NC}"

    # Reddit client benchmarks
    if [ -f "reddit/reddit_bench_test.go" ]; then
        run_benchmarks "./reddit" "Reddit Client"
    fi

    # Scenario benchmarks
    if [ -f "reddit/benchmark_test.go" ]; then
        run_benchmarks "./reddit" "Scenarios"
    fi
fi

# Run comparative benchmarks
if [ "$RUN_COMPARATIVE" = true ]; then
    echo -e "${GREEN}=== Comparative Benchmarks ===${NC}"

    if [ -f "benchmarks/comparative/reddit_comparison_test.go" ]; then
        run_benchmarks "./benchmarks/comparative" "Comparative"
    fi
fi

echo ""
echo -e "${GREEN}=====================================${NC}"
echo -e "${GREEN}  Benchmarks Complete!              ${NC}"
echo -e "${GREEN}=====================================${NC}"

# Run comparison if requested
if [ -n "$COMPARE_FILE" ] && [ "$SAVE_RESULTS" = true ]; then
    echo ""
    echo -e "${GREEN}Comparing with baseline: $COMPARE_FILE${NC}"
    echo ""
    benchstat "$COMPARE_FILE" "$OUTPUT_FILE"
fi

if [ "$SAVE_RESULTS" = true ]; then
    echo ""
    echo -e "${GREEN}Results saved to: $OUTPUT_FILE${NC}"
    echo -e "${YELLOW}To compare with this baseline later, run:${NC}"
    echo -e "  ./benchmarks/run.sh --save --compare $OUTPUT_FILE"
fi
