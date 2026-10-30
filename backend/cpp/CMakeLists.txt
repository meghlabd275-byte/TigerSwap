cmake_minimum_required(VERSION 3.16)
project(TigerSwapHftEngine VERSION 1.0.0 LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)
set(CMAKE_EXPORT_COMPILE_COMMANDS ON)

if(NOT CMAKE_BUILD_TYPE)
    set(CMAKE_BUILD_TYPE Release)
endif()

find_package(Threads REQUIRED)
find_package(OpenSSL REQUIRED)

add_library(tigerswap_engine_core STATIC
    src/wal.cpp
    src/crypto.cpp
    src/network.cpp
)
target_include_directories(tigerswap_engine_core PUBLIC ${CMAKE_CURRENT_SOURCE_DIR}/include)
target_link_libraries(tigerswap_engine_core PUBLIC Threads::Threads OpenSSL::Crypto)

# Latency-sensitive translation units get aggressive optimisation; the tests
# link the same code so what is tested is what ships.
target_compile_options(tigerswap_engine_core PRIVATE
    -O3
    -Wall
    -Wextra
    -Wpedantic
)

add_executable(tigerswap_engine src/main.cpp)
target_link_libraries(tigerswap_engine PRIVATE tigerswap_engine_core)

enable_testing()

add_executable(tigerswap_tests tests/engine_tests.cpp)
target_link_libraries(tigerswap_tests PRIVATE tigerswap_engine_core)

# ThreadSanitizer needs a flat address space and aborts with "unexpected memory
# mapping" under the default PIE + ASLR layout, so under TSan the test runs
# through `setarch -R` when that tool is available.
if(CMAKE_CXX_FLAGS MATCHES "fsanitize=thread")
    find_program(SETARCH_EXECUTABLE setarch)
endif()

if(SETARCH_EXECUTABLE)
    add_test(NAME tigerswap_tests
        COMMAND ${SETARCH_EXECUTABLE} ${CMAKE_SYSTEM_PROCESSOR} -R $<TARGET_FILE:tigerswap_tests>)
else()
    add_test(NAME tigerswap_tests COMMAND tigerswap_tests)
endif()

install(TARGETS tigerswap_engine RUNTIME DESTINATION bin)
