#pragma once
#ifndef _CFGO_UTILS_H_
#define _CFGO_UTILS_H_
#include <string>
#include <exception>

namespace cfgo {
    std::string what(const std::exception_ptr &eptr = std::current_exception());
}

#endif