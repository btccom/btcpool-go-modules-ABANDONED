# 用于生成安装包

cmake_minimum_required (VERSION 2.6)

project (btcpool-switcher NONE)

# install prefix
if(NOT INSTALL_PREFIX)
    set(INSTALL_PREFIX "/work/golang/stratumSwitcher")
endif()
message("-- Package Install Prefix (-DINSTALL_PREFIX=<path>): ${INSTALL_PREFIX}")
set(CMAKE_INSTALL_PREFIX ${INSTALL_PREFIX})

# GOPATH
set(GOPATH $ENV{GOPATH})
if(NOT GOPATH)
    set(GOPATH "/work/golang")
endif()

install(PROGRAMS
            ${GOPATH}/bin/stratumSwitcher
        DESTINATION
            ${INSTALL_PREFIX})

install(FILES
            config.default.json
            supervisor-switcher.conf
        DESTINATION
            ${INSTALL_PREFIX})

###
# Debian/Ubuntu software package
###
message("-- Generate Debian/Ubuntu software package")
set(CPACK_GENERATOR "DEB")
set(CPACK_PACKAGE_NAME "btcpool-switcher")
set(CPACK_DEBIAN_PACKAGE_MAINTAINER "YihaoPeng yihao.peng@bitmain.com")

include(CPack)
