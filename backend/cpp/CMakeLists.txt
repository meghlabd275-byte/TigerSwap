cmake_minimum_required(VERSION 3.15)
project(TigerSwapTradingEngine VERSION 1.0.0 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_EXPORT_COMPILE_COMMANDS ON)

# Find required packages
find_package(Threads REQUIRED)
find_package(OpenSSL REQUIRED)
find_package(Boost 1.75 REQUIRED COMPONENTS system thread filesystem)

# Include directories
include_directories(${CMAKE_CURRENT_SOURCE_DIR}/include)

# Source files
set(SOURCES
    src/main.cpp
    src/trading_engine.cpp
    src/order_book.cpp
    src/matching_engine.cpp
    src/risk_manager.cpp
    src/price_feed.cpp
    src/network.cpp
)

# Create executable
add_executable(tigerswap_engine ${SOURCES})

# Link libraries
target_link_libraries(tigerswap_engine
    Threads::Threads
    OpenSSL::SSL
    OpenSSL::Crypto
    Boost::system
    Boost::thread
    Boost::filesystem
)

# Compiler optimizations for ultra-low latency
target_compile_options(tigerswap_engine PRIVATE
    -O3
    -march=native
    -mtune=native
    -ffast-math
    -funroll-loops
    -flto
)

# Installation
install(TARGETS tigerswap_engine DESTINATION bin)
install(FILES config/config.json DESTINATION etc/tigerswap)

message(STATUS "TigerSwap Trading Engine configured for ultra-low latency")
