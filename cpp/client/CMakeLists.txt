cmake_minimum_required(VERSION 3.21)
project(cfgoclient)

set(SRC_PATH "${CMAKE_CURRENT_SOURCE_DIR}/src")
set(H_PUBLIC "${SRC_PATH}/public")
set(H_PUBLIC_PATH "${H_PUBLIC}/cfgo")
set(MY_PUBLIC_HEADERS
    "${H_PUBLIC_PATH}/configuration.hpp"
    "${H_PUBLIC_PATH}/client.hpp"
)
set(MY_SOURCES
    "${SRC_PATH}/configuration.cpp"
    "${SRC_PATH}/client.cpp"
)

add_library(cfgoclient ${MY_SOURCES} ${MY_PUBLIC_HEADERS})
target_include_directories(cfgoclient PUBLIC ${H_PUBLIC})

find_package(Boost REQUIRED)

find_package(sioclient CONFIG REQUIRED)
target_link_libraries(cfgoclient PRIVATE sioclient::sioclient sioclient::sioclient_tls)

find_package(unofficial-usrsctp CONFIG REQUIRED) # must add it when linux
find_package(LibDataChannel CONFIG REQUIRED)
target_link_libraries(cfgoclient PRIVATE LibDataChannel::LibDataChannelStatic)