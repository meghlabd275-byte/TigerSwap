cmake_minimum_required(VERSION 3.20)
project(TigerMatchingEngine VERSION 1.0.0 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

# Compiler flags for high performance
if(CMAKE_CXX_COMPILER_ID MATCHES "GNU|Clang")
    set(CMAKE_CXX_FLAGS_RELEASE "${CMAKE_CXX_FLAGS_RELEASE} -O3 -march=native -mtune=native")
    set(CMAKE_CXX_FLAGS "${CMAKE_CXX_FLAGS} -Wall -Wextra")
elseif(MSVC)
    set(CMAKE_CXX_FLAGS_RELEASE "${CMAKE_CXX_FLAGS_RELEASE} /O2 /Ob2 /fp:fast")
endif()

# Find dependencies
find_package(Threads REQUIRED)

# Header-only library option
option(TIGER_HEADER_ONLY "Build as header-only library" OFF)

if(TIGER_HEADER_ONLY)
    # Header-only: no compilation needed
    message(STATUS "Building TigerMatchingEngine as header-only library")
else()
    # Source files
    set(SOURCES
        src/main.cpp
        src/matching_engine.cpp
    )
    
    # Create library
    add_library(tiger_matching_engine STATIC ${SOURCES})
    
    target_include_directories(tiger_matching_engine PUBLIC 
        $<BUILD_INTERFACE:${CMAKE_CURRENT_SOURCE_DIR}/include>
        $<INSTALL_INTERFACE:include>
    )
    
    target_link_libraries(tiger_matching_engine PRIVATE Threads::Threads)
    
    # Enable LTO for Release builds
    if(CMAKE_BUILD_TYPE STREQUAL "Release")
        include(CheckIPOSupported)
        check_ipo_supported(RESULT ipo_supported)
        if(ipo_supported)
            set_target_properties(tiger_matching_engine PROPERTIES
                INTERPROCEDURAL_OPTIMIZATION TRUE
            )
        endif()
    endif()
    
    # Installation
    include(GNUInstallDirs)
    install(TARGETS tiger_matching_engine
        ARCHIVE DESTINATION ${CMAKE_INSTALL_LIBDIR}
        LIBRARY DESTINATION ${CMAKE_INSTALL_LIBDIR}
    )
    install(DIRECTORY include/ 
        DESTINATION ${CMAKE_INSTALL_INCLUDEDIR}
        FILES_MATCHING PATTERN "*.hpp"
    )
endif()

# Tests
enable_testing()
add_subdirectory(tests)

# Benchmarks
option(TIGER_BUILD_BENCHMARKS "Build benchmarks" OFF)
if(TIGER_BUILD_BENCHMARKS)
    add_subdirectory(benchmarks)
endif()

message(STATUS "TigerMatchingEngine configuration complete")
message(STATUS "  C++ Standard: ${CMAKE_CXX_STANDARD}")
message(STATUS "  Build type: ${CMAKE_BUILD_TYPE}")
