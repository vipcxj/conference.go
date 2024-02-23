#include <assert.h>
#include <exception>
#include "cfgo/track.hpp"
#include "cfgo/subscribation.hpp"
#include "cfgo/defer.hpp"
#include "impl/client.hpp"
#include "rtc/rtc.hpp"
#include "boost/lexical_cast.hpp"
#include "boost/uuid/uuid_io.hpp"
#include "boost/uuid/uuid_generators.hpp"
#include "asiochan/asiochan.hpp"

namespace cfgo
{
    namespace impl
    {

        Client::Client(const Configuration &config) : Client(config, std::make_shared<asio::io_context>(), true)
        {
        }

        Client::Client(const Configuration &config, const CtxPtr &io_ctx) : Client(config, io_ctx, config.m_thread_safe)
        {
        }

        Client::Client(const Configuration &config, const CtxPtr &io_ctx, bool thread_safe) : m_config(config),
                                                                                              m_client(),
                                                                                              m_peer(std::make_shared<::rtc::PeerConnection>(config.m_rtc_config)),
                                                                                              m_id(boost::lexical_cast<std::string>(boost::uuids::random_generator()())),
                                                                                              m_io_context(io_ctx),
                                                                                              m_thread_safe(thread_safe),
                                                                                              m_mutex()
        {
        }

        Client::~Client()
        {
            m_client->sync_close();
        }

        void Client::lock()
        {
            if (!m_thread_safe)
            {
                m_mutex.lock();
            }
        }

        void Client::unlock() noexcept
        {
            if (!m_thread_safe)
            {
                m_mutex.unlock();
            }
        }

        msg_ptr Client::create_auth_message() const
        {
            auto auth_msg = sio::object_message::create();
            auth_msg->get_map()["token"] = sio::string_message::create(m_config.m_token);
            auth_msg->get_map()["id"] = sio::string_message::create(m_id);
            return auth_msg;
        }

        msg_ptr create_add_cand_message(const ::rtc::Candidate &cand)
        {
            auto add_cand_msg = sio::object_message::create();
            add_cand_msg->get_map()["op"] = sio::string_message::create("add");
            auto cand_msg = sio::object_message::create();
            cand_msg->get_map()["candidate"] = sio::string_message::create(cand.candidate());
            cand_msg->get_map()["sdpMid"] = sio::string_message::create(cand.mid());
            add_cand_msg->get_map()["candidate"] = cand_msg;
            return add_cand_msg;
        }

        msg_ptr create_subscribe_message(const Pattern &pattern, const std::vector<std::string> &req_types)
        {
            auto msg = sio::object_message::create();
            msg->get_map()["op"] = sio::int_message::create(0);
            auto req_types_msg = sio::array_message::create();
            for (auto &&req_type : req_types)
            {
                req_types_msg->get_vector().push_back(sio::string_message::create(req_type));
            }
            msg->get_map()["reqTypes"] = req_types_msg;
            msg->get_map()["pattern"] = pattern.create_message();
            return msg;
        }

        msg_ptr create_unsubscribe_message(const std::string &sub_id)
        {
            auto msg = sio::object_message::create();
            msg->get_map()["op"] = sio::int_message::create(2);
            msg->get_map()["id"] = sio::string_message::create(sub_id);
            return msg;
        }

        msg_ptr create_sdp_message(int sdp_id, const ::rtc::Description &desc)
        {
            auto sdp_msg = sio::object_message::create();
            sdp_msg->get_map()["type"] = sio::string_message::create(desc.typeString());
            sdp_msg->get_map()["sdp"] = sio::string_message::create(desc);
            sdp_msg->get_map()["mid"] = sio::int_message::create(sdp_id);
            return sdp_msg;
        }

        auto Client::accquire(close_chan &close_chan) -> asio::awaitable<bool>
        {
            bool success = false;
            busy_chan ch{};
            bool ch_added = false;
            while (true)
            {
                {
                    Guard g(this);
                    if (!m_busy)
                    {
                        m_busy = true;
                        success = true;
                        if (ch_added)
                        {
                            m_busy_chans.erase(std::remove(m_busy_chans.begin(), m_busy_chans.end(), ch), m_busy_chans.end());
                        }
                    }
                    else if (!ch_added)
                    {
                        ch_added = true;
                        m_busy_chans.push_back(ch);
                    }
                }
                if (success)
                {
                    co_return true;
                }
                auto &&result = co_await asiochan::select(
                    asiochan::ops::read(ch, close_chan));
                if (result.received_from(close_chan))
                {
                    co_return false;
                }
            }
        }

        void Client::release()
        {
            Guard g(this);
            m_busy = false;
            for (auto &ch : m_busy_chans)
            {
                write_ch(ch);
            }
        }

        template <class T>
        concept construct_with_msg_ptr = requires(msg_ptr a) {
            std::make_shared<T>(a);
        };
        template <class T>
        concept convertible_from_msg_ptr = std::is_convertible_v<std::string, T> || std::is_convertible_v<int, T>;

        template <construct_with_msg_ptr T>
        auto get_msg_object_field(msg_ptr msg, std::string field) -> std::shared_ptr<T>
        {
            if (!msg)
            {
                return nullptr;
            }
            auto iter = msg->get_map().find(field);
            if (iter == msg->get_map().end() || !iter->second)
            {
                return nullptr;
            }
            if constexpr (std::is_same_v<std::decay_t<T>, sio::message>)
            {
                return iter->second;
            }
            else
            {
                return std::make_shared<T>(iter->second);
            }
        }

        template <construct_with_msg_ptr T>
        void get_msg_object_array_field(msg_ptr msg, std::string field, std::vector<std::shared_ptr<T>> &result)
        {
            if (!msg)
            {
                return;
            }
            auto iter = msg->get_map().find(field);
            if (iter == msg->get_map().end() || !iter->second)
            {
                return;
            }
            for (auto &&m : iter->second->get_vector())
            {
                if (m)
                {
                    result.push_back(std::make_shared<T>(m));
                }
                else
                {
                    result.push_back(nullptr);
                }
            }
        }

        template <convertible_from_msg_ptr T>
        auto constexpr cast_msg_to_base(const msg_ptr msg) -> std::optional<T>
        {
            if constexpr (std::is_convertible_v<std::string, T>)
            {
                return msg->get_string();
            }
            else if constexpr (std::is_convertible_v<int, T>)
            {
                return msg->get_int();
            }
            else
            {
                return std::nullopt;
            }
        }

        template <convertible_from_msg_ptr T>
        auto get_msg_base_field(msg_ptr msg, std::string field) -> std::optional<T>
        {
            if (!msg)
            {
                return std::nullopt;
            }
            auto iter = msg->get_map().find(field);
            if (iter == msg->get_map().end() || !iter->second)
            {
                return std::nullopt;
            }
            return cast_msg_to_base<T>(iter->second);
        }

        auto to_description(msg_ptr msg) -> std::optional<::rtc::Description>
        {
            auto &&sdp = get_msg_base_field<std::string>(msg, "sdp");
            auto &&type = get_msg_base_field<std::string>(msg, "type");
            if (!sdp || !type)
            {
                return std::nullopt;
            }
            else
            {
                return ::rtc::Description{sdp.value(), type.value()};
            }
        }

        void Client::emit(const std::string &evt, msg_ptr msg)
        {
            m_client->socket()->emit(evt, std::move(msg));
        }

        auto Client::emit_with_ack(const std::string &evt, msg_ptr msg, close_chan &close_chan) const -> asio::awaitable<std::optional<msg_ptr>>
        {
            auto ack_ch = std::make_shared<msg_chan>();
            msg_chan_weak_ptr weak_ack_ch = ack_ch;
            auto weak_self = weak_from_this();
            m_client->socket()->emit(evt, msg, [weak_self, weak_ack_ch](auto &&ack_msgs)
                                     {
            if (ack_msgs.size() > 0)
            {
                auto ack_msg = ack_msgs[0];
                if (auto ack_ch = weak_ack_ch.lock())
                {
                    if (auto self = weak_self.lock())
                    {
                        asio::co_spawn(*(self->m_io_context), ack_ch->write(ack_msg), asio::detached);
                    }
                }
            } });
            auto result = co_await asiochan::select(
                asiochan::ops::read(*ack_ch),
                asiochan::ops::read(close_chan));
            if (result.received<msg_ptr>())
            {
                co_return result.get_received<msg_ptr>();
            }
            else
            {
                co_return std::nullopt;
            }
        }

        auto Client::wait_for_msg(const std::string &evt, MsgChanner &msg_channer, close_chan &close_chan, std::function<bool(msg_ptr)> cond) -> asio::awaitable<std::optional<msg_ptr>>
        {
            auto &&ch = msg_channer.chan(evt);
            msg_ptr msg = nullptr;
            while (true)
            {
                auto result = co_await asiochan::select(
                    asiochan::ops::read(ch),
                    asiochan::ops::read(close_chan));
                if (result.received<void>())
                {
                    msg_channer.release(evt);
                    co_return std::nullopt;
                }
                msg = result.get_received<msg_ptr>();
                if (cond(msg))
                {
                    msg_channer.release(evt);
                    break;
                }
            }
            co_return msg;
        }

        auto Client::subscribe(const Pattern &pattern, const std::vector<std::string> &req_types, close_chan &close_chan) -> asio::awaitable<cfgo::Subscribation::Ptr>
        {
            if (co_await accquire(close_chan))
            {
                DEFER({
                    release();
                });
                m_client->connect(m_config.m_signal_url, create_auth_message());
                DEFERS_WHEN_FAIL(defers);
                std::mutex cand_mux;
                std::vector<msg_ptr> cands;
                bool remoted = false;
                m_peer->onLocalCandidate([this](auto &&cand)
                                         { emit("candidate", create_add_cand_message(cand)); });
                DEFER({
                    m_peer->onLocalCandidate(nullptr);
                });
                asiochan::channel<::rtc::PeerConnection::State> peer_state_chan{};
                m_peer->onStateChange([this, &peer_state_chan](auto &&state)
                                      {
                switch (state)
                {
                case ::rtc::PeerConnection::State::Failed:
                case ::rtc::PeerConnection::State::Closed:
                case ::rtc::PeerConnection::State::Connected:
                    write_ch(peer_state_chan, state);
                    break;
                default:
                    break;
                } });
                DEFER({
                    m_peer->onStateChange(nullptr);
                });
                m_client->socket()->on("candidate", [this, &remoted, &cands, &cand_mux](auto &&evt)
                                       {
                if (evt.need_ack())
                {
                    evt.put_ack_message(sio::message::list("ack"));
                }
                std::lock_guard guard(cand_mux);
                if (!remoted)
                {
                    cands.push_back(evt.get_message());
                }
                else
                {
                    this->add_candidate(evt.get_message());
                } });
                DEFER({
                    m_client->socket()->off("candidate");
                });
                MsgChanner msg_channer(this);
                msg_channer.prepare("sdp");
                msg_channer.prepare("subscribed");
                auto sub_msg = create_subscribe_message(pattern, req_types);
                auto sub_res = co_await emit_with_ack("subscribe", create_subscribe_message(pattern, req_types), close_chan);
                if (!sub_res)
                {
                    co_return nullptr;
                }
                auto sub_id = get_msg_base_field<std::string>(sub_res.value(), "id");
                if (!sub_id)
                {
                    throw std::runtime_error("no id found on subscribe ack msg.");
                }
                defers.add_defer([this, sub_id = sub_id.value()]()
                                 { emit("subscribe", create_unsubscribe_message(std::move(sub_id))); });

                auto subed_msg = co_await wait_for_msg("subscribed", msg_channer, close_chan, [&sub_id](auto &&msg)
                                                       { return get_msg_base_field<std::string>(msg, "subId") == sub_id; });
                if (!subed_msg)
                {
                    co_return nullptr;
                }
                auto sdp_id = get_msg_base_field<int>(subed_msg.value(), "sdpId");
                if (!sdp_id)
                {
                    throw std::runtime_error("no sdpId found on subscribed msg.");
                }
                auto pub_id = get_msg_base_field<std::string>(subed_msg.value(), "pubId");
                if (!pub_id)
                {
                    throw std::runtime_error("no pubId found on subscribed msg.");
                }
                auto sub_ptr = std::make_shared<cfgo::Subscribation>(sub_id.value(), pub_id.value());
                get_msg_object_array_field<cfgo::Track>(subed_msg.value(), "tracks", sub_ptr->tracks());
                if (sub_ptr->tracks().empty())
                {
                    defers.success();
                    co_return sub_ptr;
                }
                std::vector<TrackPtr> uncompleted_tracks(sub_ptr->tracks());
                asiochan::channel<void> tracks_ch{};
                m_peer->onTrack([&uncompleted_tracks, &tracks_ch, this](auto &&track) mutable
                                {
                auto&& iter = std::partition(uncompleted_tracks.begin(), uncompleted_tracks.end(), [track = std::move(track)](const TrackPtr& t) -> bool {
                    return t->bind_id() == track->mid();
                });
                if (iter != uncompleted_tracks.end())
                {
                    (*iter)->track() = track;
                    uncompleted_tracks.erase(iter, uncompleted_tracks.end());
                }
                if (uncompleted_tracks.empty())
                {
                    this->write_ch(tracks_ch);
                } });
                DEFER({
                    m_peer->onTrack(nullptr);
                });

                auto sdp_msg = co_await wait_for_msg("sdp", msg_channer, close_chan, [sdp_id](auto &&msg)
                                                     { return get_msg_base_field<int>(msg, "mid") == sdp_id; });
                if (!sdp_msg)
                {
                    co_return nullptr;
                }
                auto &&desc = to_description(sdp_msg.value());
                if (!desc)
                {
                    throw std::runtime_error("bad sdp msg");
                }
                m_peer->setRemoteDescription(desc.value());
                {
                    std::lock_guard guard(cand_mux);
                    remoted = true;
                    for (auto &&m : cands)
                    {
                        add_candidate(m);
                    }
                }
                auto &&state_res = co_await asiochan::select(
                    asiochan::ops::read(peer_state_chan),
                    asiochan::ops::read(close_chan));
                if (state_res.matches(close_chan))
                {
                    co_return nullptr;
                }
                auto state = state_res.get_received<::rtc::PeerConnection::State>();
                if (state != ::rtc::PeerConnection::State::Connected)
                {
                    co_return nullptr;
                }

                auto &&res = co_await asiochan::select(
                    asiochan::ops::read(tracks_ch, close_chan));
                if (res.matches(close_chan))
                {
                    co_return nullptr;
                }
                else
                {
                    defers.success();
                    co_return sub_ptr;
                }
            }
            else
            {
                co_return nullptr;
            }
        }

        auto Client::_unsubscribe(const std::string &sub_id, close_chan &close_chan) -> asio::awaitable<std::optional<bool>>
        {
            MsgChanner msg_channer(this);
            auto res = co_await emit_with_ack("subscribe", create_unsubscribe_message(sub_id), close_chan);
            if (!res)
            {
                co_return std::nullopt;
            }
            co_return get_msg_base_field<std::string>(res.value(), "id") == sub_id;
        }

        void Client::add_candidate(const msg_ptr &msg)
        {
            auto &&op = get_msg_base_field<std::string>(msg, "op");
            if (!op)
            {
                throw std::runtime_error("no op found on candidate msg.");
            }
            if (op == "add")
            {
                msg_ptr candidate_msg = get_msg_object_field<sio::message>(msg, "candidate");
                auto &&candidate = get_msg_base_field<std::string>(candidate_msg, "candidate");
                auto &&mid = get_msg_base_field<std::string>(candidate_msg, "sdpMid");
                m_peer->addRemoteCandidate(::rtc::Candidate{candidate.value_or(""), mid.value_or("")});
            }
        }

        Client::MsgChanner::MsgChanner(Client *const client) : m_client(client) {}

        void Client::MsgChanner::prepare(const std::string &evt, msg_ptr const &ack)
        {
            if (m_chan_map.contains(evt))
            {
                return;
            }
            msg_chan ch{};
            m_chan_map[evt] = ch;
            m_client->m_client->socket()->on(evt, [ch, c = m_client, ack = std::move(ack)](auto &&evt) mutable
                                             {
            if (evt.need_ack())
            {
                evt.put_ack_message(sio::message::list(ack));
            }
            c->write_ch(ch, evt.get_message()); });
        }

        void Client::MsgChanner::release(const std::string &evt)
        {
            m_client->m_client->socket()->off(evt);
            m_chan_map.erase(evt);
        }

        msg_chan &Client::MsgChanner::chan(const std::string &evt)
        {
            if (!m_chan_map.contains(evt))
            {
                throw std::logic_error("evt " + evt + " has not been prepared.");
            }
            return m_chan_map.at(evt);
        }

        const msg_chan &Client::MsgChanner::chan(const std::string &evt) const
        {
            if (!m_chan_map.contains(evt))
            {
                throw std::logic_error("evt " + evt + " has not been prepared.");
            }
            return m_chan_map.at(evt);
        }

        Client::MsgChanner::~MsgChanner()
        {
            for (auto &&p : m_chan_map)
            {
                m_client->m_client->socket()->off(p.first);
            }
        }
    }
}