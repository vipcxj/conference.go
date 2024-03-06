#pragma once
#ifndef _CFGO_UTILS_H_
#define _CFGO_UTILS_H_
#include <string>
#include <exception>
#include <memory>
#include "cfgo/alias.hpp"

namespace cfgo
{
    std::string what(const std::exception_ptr &eptr = std::current_exception());

    template <typename T>
    using impl_ptr = std::shared_ptr<T>;
    template <typename T>
    class ImplBy
    {
    public:
        ImplBy(impl_ptr<T> impl) : mImpl(std::move(impl)) {}
        template <typename... Args>
        ImplBy(Args... args) : mImpl(std::make_shared<T>(std::forward<Args>(args)...)) {}
        ImplBy(ImplBy<T> &&cc) { *this = std::move(cc); }
        ImplBy(const ImplBy<T> &) = delete;

        virtual ~ImplBy() = default;

        ImplBy &operator=(ImplBy<T> &&cc)
        {
            mImpl = std::move(cc.mImpl);
            return *this;
        };
        ImplBy &operator=(const ImplBy<T> &) = delete;

    protected:
        impl_ptr<T> & impl() noexcept { return mImpl; }
        const impl_ptr<T> & impl() const noexcept { return mImpl; }

    private:
        impl_ptr<T> mImpl;
    };
}

#endif