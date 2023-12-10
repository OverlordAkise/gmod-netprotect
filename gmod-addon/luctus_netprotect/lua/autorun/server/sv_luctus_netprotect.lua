--Luctus Netprotect
--Made by OverlordAkise

LUCTUS_NETPROTECT_ACTIVE = LUCTUS_NETPROTECT_ACTIVE or false

LUCTUS_NETPROTECT_CONNECT_CACHE = LUCTUS_NETPROTECT_CONNECT_CACHE or {}

hook.Add("PlayerConnect","luctus_netprotect",function(name, ip)
    if not LUCTUS_NETPROTECT_ACTIVE then return end
    print("[luctus_netprotect] PlayerConnect",name,ip)
    ip = string.Split(ip,":")[1]
    LUCTUS_NETPROTECT_CONNECT_CACHE[name] = ip
    local ret = HTTP({
        failed = function(failMessage) print("[luctus_netprotect] Error during IP adding of player! (/add)") error(failMessage) end,
        success = function(httpcode,body,headers) print("[luctus_netprotect] IPs of joining player added!",httpcode,body) end, 
        method = "POST",
        url = "http://localhost:3531/add",
        body = util.TableToJSON({["ip"]=ip}),
        type = "application/json; charset=utf-8",
        timeout = 3
    })
    if not ret then
        error("ERROR: Couldn't make http request to luctus netprotect /add!")
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

hook.Add("PlayerInitialSpawn","luctus_netprotect_cleanup",function(ply)
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
        failed = function(failMessage) print("[luctus_netprotect] Error during IP delete of player! (/del)") error(failMessage) end,
        success = function(httpcode,body,headers) print("[luctus_netprotect] IPs of leaving player removed!",httpcode,body) end, 
        method = "POST",
        url = "http://localhost:3531/del",
        body = util.TableToJSON({["ip"]=ip}),
        type = "application/json; charset=utf-8",
        timeout = 3
    })
    if not ret then
        error("ERROR: Couldn't make http request to luctus netprotect /del!")
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
        failed = function(failMessage) print("[luctus_netprotect] Error during IP adding of current players! (/addmany)") error(failMessage) end,
        success = function(httpcode,body,headers) print("[luctus_netprotect] IPs of current players added!",httpcode,body) end, 
        method = "POST",
        url = "http://localhost:3531/addmany",
        body = util.TableToJSON({["ips"]=ips}),
        type = "application/json; charset=utf-8",
        timeout = 3
    })
    if not ret then
        error("ERROR: Couldn't make http request to luctus netprotect /addmany!")
    end
end

function LuctusNetProtectEnable(callback)
    local ret = HTTP({
        failed = function(failMessage) print("[luctus_netprotect] Error during start! (/start)") error(failMessage) end,
        success = function(httpcode,body,headers)
            print("[luctus_netprotect] activated!",httpcode,body)
            if callback and isfunction(callback) then
                callback()
            end
        end, 
        method = "POST",
        url = "http://localhost:3531/start",
        body = "",
        type = "application/json; charset=utf-8",
        timeout = 3
    })
    if not ret then
        error("ERROR: Couldn't make http request to luctus netprotect /start!")
    end
end

function LuctusNetprotectDisable()
    if not LUCTUS_NETPROTECT_ACTIVE then
        print("ERROR: Luctus Netprotect is not active!")
        return
    end
    LUCTUS_NETPROTECT_ACTIVE = false
    local ret = HTTP({
        failed = function(failMessage) print("[luctus_netprotect] Error during stop! (/stop)") error(failMessage) end,
        success = function(httpcode,body,headers) print("[luctus_netprotect] deactivated!",httpcode,body) end, 
        method = "POST",
        url = "http://localhost:3531/stop",
        body = "",
        type = "application/json; charset=utf-8",
        timeout = 3
    })
    if not ret then
        error("ERROR: Couldn't make http request to luctus netprotect /stop!")
    end
end

hook.Add("InitPostEntity","luctus_netprotect_cleanup",function()
    print("[luctus_netprotect] Setting to off for cleanup")
    local ret = HTTP({
        failed = function(failMessage) print("[luctus_netprotect] Error during stop! (/stop)") error(failMessage) end,
        success = function(httpcode,body,headers) print("[luctus_netprotect] cleaned up!",httpcode,body) end, 
        method = "POST",
        url = "http://localhost:3531/stop",
        body = "",
        type = "application/json; charset=utf-8",
        timeout = 3
    })
    if not ret then
        error("ERROR: Couldn't make http request to luctus netprotect /stop!")
    end
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

print("[luctus_netprotect] sv loaded")
