cmake_minimum_required(VERSION 3.21)
project(cfgoclient)

set(CMAKE_CXX_STANDARD 20)
set(CMAKE_CXX_STANDARD_REQUIRED ON)

set(SRC_PATH "${CMAKE_CURRENT_SOURCE_DIR}/src")
set(H_IMPL "${SRC_PATH}/impl")
set(H_PUBLIC "${SRC_PATH}/public")
set(H_PUBLIC_PATH "${H_PUBLIC}/cfgo")
set(MY_PUBLIC_HEADERS
    "${H_PUBLIC_PATH}/configuration.hpp"
    "${H_PUBLIC_PATH}/client.hpp"
    "${H_PUBLIC_PATH}/subscribation.hpp"
    "${H_PUBLIC_PATH}/track.hpp"
    "${H_PUBLIC_PATH}/alias.hpp"
    "${H_PUBLIC_PATH}/pattern.hpp"
    "${H_PUBLIC_PATH}/error.hpp"
)
set(H_PRIVATE "${SRC_PATH}/private")
set(H_PRIVATE_PATH "${H_PRIVATE}/cfgo")
set(MY_PRIVATE_HEADERS
    "${H_PRIVATE_PATH}/cofuture.hpp"
    "${H_PRIVATE_PATH}/cothread.hpp"
    "${H_PRIVATE_PATH}/coevent.hpp"
    "${H_PRIVATE_PATH}/move_only_function.hpp"
    "${H_PRIVATE_PATH}/utils.hpp"
    "${H_PRIVATE_PATH}/defer.hpp"
    "${H_IMPL}/client.hpp"
    "${H_IMPL}/track.hpp"
    "${H_IMPL}/subscribation.hpp"
)
set(MY_SOURCES
    "${SRC_PATH}/configuration.cpp"
    "${SRC_PATH}/client.cpp"
    "${SRC_PATH}/track.cpp"
    "${SRC_PATH}/subscribation.cpp"
    "${SRC_PATH}/error.cpp"
    "${SRC_PATH}/coevent.cpp"
    "${SRC_PATH}/utils.cpp"
    "${H_IMPL}/client.cpp"
    "${H_IMPL}/track.cpp"
    "${H_IMPL}/subscribation.cpp"
)
set(MY_TEST_PATH "${SRC_PATH}/test")

add_library(cfgoclient ${MY_SOURCES} ${MY_PUBLIC_HEADERS} ${MY_PRIVATE_HEADERS})
target_include_directories(cfgoclient PUBLIC ${H_PUBLIC})
target_include_directories(cfgoclient PRIVATE ${H_PRIVATE} ${SRC_PATH})

find_package(Boost REQUIRED)

find_package(sioclient CONFIG REQUIRED)
target_link_libraries(cfgoclient PRIVATE sioclient::sioclient sioclient::sioclient_tls)

find_package(LibDataChannel CONFIG REQUIRED)
if(WIN32)
    target_link_libraries(cfgoclient PRIVATE LibDataChannel::LibDataChannel)
else()
    target_link_libraries(cfgoclient PRIVATE LibDataChannel::LibDataChannelStatic)
endif(WIN32)

find_path(ASIOCHAN_INCLUDE_DIRS "asiochan/asiochan.hpp")
target_compile_definitions(cfgoclient PRIVATE ASIOCHAN_USE_STANDALONE_ASIO)
target_include_directories(cfgoclient PRIVATE ${ASIOCHAN_INCLUDE_DIRS})

add_executable(test1 "${SRC_PATH}/main.cpp" ${MY_PUBLIC_HEADERS} ${MY_PRIVATE_HEADERS})

set_target_properties(test1 PROPERTIES DEBUG_POSTFIX "${CMAKE_DEBUG_POSTFIX}")


target_include_directories(test1 PUBLIC ${H_PUBLIC})
target_include_directories(test1 PRIVATE ${H_PRIVATE})
if(WIN32)
    target_link_libraries(test1 PRIVATE LibDataChannel::LibDataChannel)
else()
    target_link_libraries(test1 PRIVATE LibDataChannel::LibDataChannelStatic)
endif(WIN32)

enable_testing()

find_package(GTest CONFIG REQUIRED)

add_executable(test-coevent "${MY_TEST_PATH}/coevent.cpp" "${SRC_PATH}/coevent.cpp" "${H_PRIVATE_PATH}/coevent.hpp")
target_include_directories(test-coevent PRIVATE "${H_PRIVATE}")
find_package(asio CONFIG REQUIRED)
target_link_libraries(test-coevent PRIVATE asio::asio)
target_link_libraries(test-coevent PRIVATE GTest::gtest GTest::gtest_main GTest::gmock GTest::gmock_main)
add_test(NAME test-coevent COMMAND test-coevent)