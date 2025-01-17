package client

import (
	"ddns-watchdog/internal/common"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func Install() (err error) {
	if common.IsWindows() {
		log.Println("Windows 暂不支持安装到系统")
	} else {
		// 注册系统服务
		if Conf.CheckCycleMinutes == 0 {
			err = errors.New("设置一下 " + ConfPath + ConfFileName + " 的 check_cycle_minutes 吧")
			return
		}
		serviceContent := []byte(
			"[Unit]\n" +
				"Description=" + RunningName + " Service\n" +
				"After=network.target\n\n" +
				"[Service]\n" +
				"Type=simple\n" +
				"ExecStart=" + RunningPath + RunningName + " -c " + ConfPath +
				"\nRestart=on-failure\n" +
				"RestartSec=2\n\n" +
				"[Install]\n" +
				"WantedBy=multi-user.target\n")
		err = ioutil.WriteFile(InstallPath, serviceContent, 0664)
		if err != nil {
			return
		}
		log.Println("可以使用 systemctl 控制 " + RunningName + " 服务了")
	}
	return
}

func Uninstall() (err error) {
	if common.IsWindows() {
		log.Println("Windows 暂不支持安装到系统")
	} else {
		err = os.Remove(InstallPath)
		if err != nil {
			return
		}
		log.Println("卸载服务成功")
		log.Println("若要完全删除，请移步到 " + RunningPath + " 和 " + ConfPath + " 完全删除")
	}
	return
}

func NetworkCardRespond() (map[string]string, error) {
	networkCardInfo := make(map[string]string)

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, i := range interfaces {
		ipAddr, err2 := i.Addrs()
		if err2 != nil {
			return nil, err2
		}
		for j, addrAndMask := range ipAddr {
			// 分离 IP 和子网掩码
			addr := strings.Split(addrAndMask.String(), "/")[0]
			if strings.Contains(addr, ":") {
				addr = common.DecodeIPv6(addr)
			}
			networkCardInfo[i.Name+" "+strconv.Itoa(j)] = addr
		}
	}
	return networkCardInfo, nil
}

func GetOwnIP(enabled enable, apiUrl apiUrl, nc networkCard) (ipv4, ipv6 string, err error) {
	ncr := make(map[string]string)
	// 若需网卡信息，则获取网卡信息并提供给用户
	if enabled.NetworkCard && nc.IPv4 == "" && nc.IPv6 == "" {
		ncr, err = NetworkCardRespond()
		if err != nil {
			return
		}
		err = common.MarshalAndSave(ncr, ConfPath+NetworkCardFileName)
		if err != nil {
			return
		}
		err = errors.New("请打开 " + ConfPath + NetworkCardFileName + " 选择网卡填入 " +
			ConfPath + ConfFileName + " 的 network_card")
		return
	}

	// 若需网卡信息，则获取网卡信息
	if enabled.NetworkCard && (nc.IPv4 != "" || nc.IPv6 != "") {
		ncr, err = NetworkCardRespond()
		if err != nil {
			return
		}
	}

	// 启用 IPv4
	if enabled.IPv4 {
		// 启用网卡 IPv4
		if enabled.NetworkCard && nc.IPv4 != "" {
			ipv4 = ncr[nc.IPv4]
			if ipv4 == "" {
				err = errors.New("IPv4 选择了不存在的网卡")
				return
			}
		} else {
			// 使用 API 获取 IPv4
			if apiUrl.IPv4 == "" {
				apiUrl.IPv4 = common.DefaultAPIUrl
			}
			res, err2 := http.Get(apiUrl.IPv4)
			if err2 != nil {
				err = err2
				return
			}
			defer func(Body io.ReadCloser) {
				t := Body.Close()
				if t != nil {
					err = t
				}
			}(res.Body)
			recvJson, err2 := ioutil.ReadAll(res.Body)
			if err2 != nil {
				err = err2
				return
			}
			var ipInfo common.PublicInfo
			err = json.Unmarshal(recvJson, &ipInfo)
			if err != nil {
				return
			}
			ipv4 = ipInfo.IP
		}
		if strings.Contains(ipv4, ":") {
			err = errors.New("获取到的 IPv4 格式错误，意外获取到了 " + ipv4)
			return
		}
	}

	// 启用 IPv6
	if enabled.IPv6 {
		// 启用网卡 IPv6
		if enabled.NetworkCard && nc.IPv6 != "" {
			ipv6 = ncr[nc.IPv6]
			if ipv6 == "" {
				err = errors.New("IPv6 选择了不存在的网卡")
				return
			}
		} else {
			// 使用 API 获取 IPv4
			if apiUrl.IPv6 == "" {
				apiUrl.IPv6 = common.DefaultIPv6APIUrl
			}
			res, err2 := http.Get(apiUrl.IPv6)
			if err2 != nil {
				err = err2
				return
			}
			defer func(Body io.ReadCloser) {
				t := Body.Close()
				if t != nil {
					err = t
				}
			}(res.Body)
			recvJson, err2 := ioutil.ReadAll(res.Body)
			if err2 != nil {
				err = err2
				return
			}
			var ipInfo common.PublicInfo
			err = json.Unmarshal(recvJson, &ipInfo)
			if err != nil {
				return
			}
			ipv6 = ipInfo.IP
		}
		if strings.Contains(ipv6, ":") {
			ipv6 = common.DecodeIPv6(ipv6)
		} else {
			err = errors.New("获取到的 IPv6 格式错误，意外获取到了 " + ipv6)
			return
		}
	}
	return
}
