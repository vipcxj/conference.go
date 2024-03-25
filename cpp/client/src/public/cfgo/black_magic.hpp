#ifndef _CFGO_BLACK_MAGIC_HPP_
#define _CFGO_BLACK_MAGIC_HPP_

#include <tuple>
#include <utility>

namespace cfgo
{
    namespace magic
    {
        template <bool B, typename... Ts>
        constexpr auto pick_if(Ts &&...xs)
        {
            if constexpr (B)
                return std::forward_as_tuple(std::forward<Ts>(xs)...);
            else
                return std::tuple{};
        }
    } // namespace magic

} // namespace cfgo

#endif