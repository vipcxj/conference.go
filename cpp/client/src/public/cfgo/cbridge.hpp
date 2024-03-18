#ifndef _CFGO_CBRIDGE_HPP_
#define _CFGO_CBRIDGE_HPP_

#include "cfgo/alias.hpp"
#include "cfgo/client.hpp"
#include "cfgo/pattern.hpp"
#include "cfgo/subscribation.hpp"
#include "cfgo/track.hpp"

namespace cfgo
{
    Client::CtxPtr get_execution_context(int handle);
    int emplace_execution_context(Client::CtxPtr ptr);
    void erase_execution_context(int handle);

    close_chan_ptr get_close_chan(int handle);
    int emplace_close_chan(close_chan_ptr ptr);
    void erase_close_chan(int handle);

    Client::Ptr get_client(int handle);
    int emplace_client(Client::Ptr ptr);
    void erase_client(int handle);

    Subscribation::Ptr get_subscribation(int handle);
    int emplace_subscribation(Subscribation::Ptr ptr);
    void erase_subscribation(int handle);

    Track::Ptr get_track(int handle);
    int emplace_track(Track::Ptr ptr);
    void erase_track(int handle);
} // namespace cfgo

#endif