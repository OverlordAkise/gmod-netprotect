--Luctus Netprotect
--Made by OverlordAkise

--Target for netprotect http calls
LUCTUS_NETPROTECT_HOST = "http://localhost:3531/"


-- CONFIG END

LUCTUS_NETPROTECT_ACTIVE = LUCTUS_NETPROTECT_ACTIVE or false

LUCTUS_NETPROTECT_CONNECT_CACHE = LUCTUS_NETPROTECT_CONNECT_CACHE or {}

hook.Add("PlayerConnect","luctus_netprotect",function(name, ip)
    if not LUCTUS_NETPROTECT_ACTIVE then return end
    print("[luctus_netprotect] PlayerConnect",name,ip)
    ip = string.Split(ip,":")[1]
    LUCTUS_NETPROTECT_CONNECT_CACHE[name] = ip
    local ret = HTTP({
        failed = function(failMessage) print("[luctus_netprotect] Error during IP adding of player! (/add)") ErrorNoHaltWithStack(failMessage) end,
        success = function(httpcode,body,headers) print("[luctus_netprotect] IPs of joining player added!",httpcode,body) end, 
        method = "POST",
        url = LUCTUS_NETPROTECT_HOST.."add",
        body = util.TableToJSON({["ip"]=ip}),
        type = "application/json; charset=utf-8",
        timeout = 3
    })
    if not ret then
        ErrorNoHaltWithStack("ERROR: Couldn't make http request to luctus netprotect /add!")
    end
end)

gameevent.Listen("player_disconnect")
hook.Add("player_disconnect","luctus_netprotect",function(data)
	local name = data.name
	local steamid = data.networkid
	local id = data.userid
	local bot = data.bot
	local reason = data.reason
    
    local ip = LUCTUS_NETPROTECT_CONNECT_CACHE[name]
    if ip then
        print("[luctus_netprotect] player_disconnect",name,steamid,ip)
        LuctusNetprotectRemoveIP(ip)
    end
end)

hook.Add("PlayerInitialSpawn","luctus_netprotect",function(ply)
    ply.netprotectSpawned = true
    if ply.SteamName then
        LUCTUS_NETPROTECT_CONNECT_CACHE[ply:SteamName()] = nil
    end
    LUCTUS_NETPROTECT_CONNECT_CACHE[ply:Nick()] = nil
end)

hook.Add("PlayerDisconnected","luctus_netprotect",function(ply)
    if not LUCTUS_NETPROTECT_ACTIVE then return end
    local steamid = ply:SteamID()
    local name = ply:Nick()
    local ip = string.Split(ply:IPAddress(),":")[1]
    print("[luctus_netprotect] PlayerDisconnected",ply,steamid,name,ip)
    LuctusNetprotectRemoveIP(ip)
end)

function LuctusNetprotectRemoveIP(ip)
    local ret = HTTP({
        failed = function(failMessage) print("[luctus_netprotect] Error during IP delete of player! (/del)") ErrorNoHaltWithStack(failMessage) end,
        success = function(httpcode,body,headers) print("[luctus_netprotect] IPs of leaving player removed!",httpcode,body) end, 
        method = "POST",
        url = LUCTUS_NETPROTECT_HOST.."del",
        body = util.TableToJSON({["ip"]=ip}),
        type = "application/json; charset=utf-8",
        timeout = 3
    })
    if not ret then
        ErrorNoHaltWithStack("ERROR: Couldn't make http request to luctus netprotect /del!")
    end
end

function LuctusNetprotectActivate()
    if LUCTUS_NETPROTECT_ACTIVE then
        print("ERROR: Luctus Netprotect already active!")
        return
    end
    LUCTUS_NETPROTECT_ACTIVE = true
    LuctusNetProtectEnable(LuctusNetProtectAddLivePlayers)
end

function LuctusNetProtectAddLivePlayers()
    local ips = {}
    for k,ply in ipairs(player.GetHumans()) do
        table.insert(ips,string.Split(ply:IPAddress(),":")[1])
    end
    
    local ret = HTTP({
        failed = function(failMessage) print("[luctus_netprotect] Error during IP adding of current players! (/addmany)") ErrorNoHaltWithStack(failMessage) end,
        success = function(httpcode,body,headers) print("[luctus_netprotect] IPs of current players added!",httpcode,body) end, 
        method = "POST",
        url = LUCTUS_NETPROTECT_HOST.."addmany",
        body = util.TableToJSON({["ips"]=ips}),
        type = "application/json; charset=utf-8",
        timeout = 3
    })
    if not ret then
        ErrorNoHaltWithStack("ERROR: Couldn't make http request to luctus netprotect /addmany!")
    end
end

function LuctusNetProtectEnable(callback)
    local ret = HTTP({
        failed = function(failMessage) print("[luctus_netprotect] Error during start! (/start)") ErrorNoHaltWithStack(failMessage) end,
        success = function(httpcode,body,headers)
            print("[luctus_netprotect] activated!",httpcode,body)
            if callback and isfunction(callback) then
                callback()
            end
        end, 
        method = "POST",
        url = LUCTUS_NETPROTECT_HOST.."start",
        body = "",
        type = "application/json; charset=utf-8",
        timeout = 3
    })
    if not ret then
        ErrorNoHaltWithStack("ERROR: Couldn't make http request to luctus netprotect /start!")
    end
    hook.Run("LuctusNetprotectActivated")
end

function LuctusNetprotectDisable()
    if not LUCTUS_NETPROTECT_ACTIVE then
        print("WARN: Luctus Netprotect is not active, but disable called!")
    end
    LUCTUS_NETPROTECT_ACTIVE = false
    local ret = HTTP({
        failed = function(failMessage) print("[luctus_netprotect] Error during stop! (/stop)") ErrorNoHaltWithStack(failMessage) end,
        success = function(httpcode,body,headers) print("[luctus_netprotect] deactivated!",httpcode,body) end, 
        method = "POST",
        url = LUCTUS_NETPROTECT_HOST.."stop",
        body = "",
        type = "application/json; charset=utf-8",
        timeout = 3
    })
    if not ret then
        ErrorNoHaltWithStack("ERROR: Couldn't make http request to luctus netprotect /stop!")
    end
    hook.Run("LuctusNetprotectDeactivated")
end

hook.Add("PlayerInitialSpawn","luctus_netprotect_reset",function()
    print("[luctus_netprotect] Setting to off after server restart")
    local ret = HTTP({
        failed = function(failMessage) print("[luctus_netprotect] Error during stop! (/stop)") ErrorNoHaltWithStack(failMessage) end,
        success = function(httpcode,body,headers) print("[luctus_netprotect] cleaned up!",httpcode,body) end, 
        method = "POST",
        url = LUCTUS_NETPROTECT_HOST.."stop",
        body = "",
        type = "application/json; charset=utf-8",
        timeout = 3
    })
    if not ret then
        ErrorNoHaltWithStack("ERROR: Couldn't make http request to luctus netprotect /stop!")
    end
    hook.Remove("PlayerInitialSpawn","luctus_netprotect_reset")
end)

concommand.Add("netprotect", function(ply, cmd, args, argstr)
    if IsValid(ply) then return end
    if cmd ~= "netprotect" then return end
    if argstr == "on" then
        LuctusNetprotectActivate()
    elseif argstr == "off" then
        LuctusNetprotectDisable()
    else
        print("Usage: 'netprotect on' or 'netprotect off'")
    end
end)

hook.Add("PlayerSay","luctus_netprotect",function(ply,text)
    if not ply:IsAdmin() then return end
    if text == "!netprotect on" then
        LuctusNetprotectActivate()
    end
    if text == "!netprotect off" then
        LuctusNetprotectDisable()
    end
end)

timer.Create("luctus_netprotect_auto",3,0,function()
    local plyCount = 0
    local packetLoss = 0
    local tickRate = math.Round(1 / engine.TickInterval())
    for k,ply in ipairs(player.GetHumans()) do
        if not IsValid(ply) then return end
        if not ply.netprotectSpawned then return end
        packetLoss = packetLoss + ply:PacketLoss()
        plyCount = plyCount + 1
    end
    if (plyCount*tickRate)/2 < packetLoss and not LUCTUS_NETPROTECT_ACTIVE and plyCount > 4 then
        LuctusNetprotectActivate()
        print("[luctus_netprotect] ATTENTION: packet loss detected, netprotect enabled!")
        print(Format("Packetloss: %d/%d (ply:%d,tickrate:%d)",packetLoss,plyCount*tickRate,plyCount,tickRate))
    end
end)

print("[luctus_netprotect] sv loaded")
