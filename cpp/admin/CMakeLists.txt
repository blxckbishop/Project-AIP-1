cmake_minimum_required(VERSION 3.22.1)
project(untitled1)

set(CMAKE_CXX_STANDARD 11)

set(VCPKG_INCLUDE_DIR "vcpkg_installed/x86-windows-static/include")
set(VCPKG_LIB_DIR "vcpkg_installed/x86-windows-static/lib")

include_directories(${VCPKG_INCLUDE_DIR})

add_executable(untitled1 main.cpp)

find_package(Xlnt CONFIG REQUIRED)
find_path(JWT_CPP_INCLUDE_DIRS "jwt-cpp/base.h")
find_package(httplib CONFIG REQUIRED)
find_package(nlohmann_json CONFIG REQUIRED)
find_package(OpenSSL REQUIRED)

target_include_directories(untitled1 PRIVATE ${JWT_CPP_INCLUDE_DIRS})
target_link_libraries(untitled1 OpenSSL::SSL OpenSSL::Crypto httplib::httplib xlnt::xlnt nlohmann_json::nlohmann_json )