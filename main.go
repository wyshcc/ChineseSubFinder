package main

import (
	"github.com/allanpk716/ChineseSubFinder/internal"
	"github.com/allanpk716/ChineseSubFinder/internal/dao"
	"github.com/allanpk716/ChineseSubFinder/internal/pkg"
	"github.com/allanpk716/ChineseSubFinder/internal/pkg/log_helper"
	"github.com/allanpk716/ChineseSubFinder/internal/pkg/notify_center"
	"github.com/allanpk716/ChineseSubFinder/internal/types"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
)

func init() {
	log = log_helper.GetLogger()
	config = pkg.GetConfig()
}

func main() {
	if log == nil {
		panic("log init error")
	}
	if config == nil {
		panic("read config error")
	}
	httpProxy := config.HttpProxy
	if config.UseProxy == false {
		httpProxy = ""
	}
	// 判断文件夹是否存在
	if pkg.IsDir(config.MovieFolder) == false {
		log.Errorln("MovieFolder not found")
		return
	}
	if pkg.IsDir(config.SeriesFolder) == false {
		log.Errorln("SeriesFolder not found")
		return
	}
	// ------ 数据库相关操作 Start ------
	err := dao.InitDb()
	if err != nil {
		log.Errorln("dao.InitDb()", err)
		return
	}
	// ------ 数据库相关操作 End ------

	// ------ Hot Fix Start ------

	// ------ Hot Fix End ------

	// 初始化通知缓存模块
	notify_center.Notify = notify_center.NewNotifyCenter(config.WhenSubSupplierInvalidWebHook)

	log.Infoln("MovieFolder:", config.MovieFolder)
	log.Infoln("SeriesFolder:", config.SeriesFolder)

	// ReloadBrowser 提前把浏览器下载好
	pkg.ReloadBrowser()

	// 任务还没执行完，下一次执行时间到来，下一次执行就跳过不执行
	c := cron.New(cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)))
	// 定时器
	entryID, err := c.AddFunc("@every "+config.EveryTime, func() {

		DownLoadStart(httpProxy)
	})
	if err != nil {
		log.Errorln("cron entryID:", entryID, "Error:", err)
		return
	}
	log.Infoln("First Time Download Start")

	DownLoadStart(httpProxy)

	log.Infoln("First Time Download End")

	c.Start()
	// 阻塞
	select {}
}

func DownLoadStart(httpProxy string) {
	defer func() {
		log.Infoln("Download One End...")
		notify_center.Notify.Send()
	}()
	notify_center.Notify.Clear()

	// 下载实例
	downloader := internal.NewDownloader(types.ReqParam{
		HttpProxy:                     httpProxy,
		DebugMode:                     config.DebugMode,
		SaveMultiSub:                  config.SaveMultiSub,
		Threads:                       config.Threads,
		SubTypePriority:               config.SubTypePriority,
		WhenSubSupplierInvalidWebHook: config.WhenSubSupplierInvalidWebHook,
		EmbyConfig:                    config.EmbyConfig,
	})

	log.Infoln("Download One Started...")

	// 刷新 Emby 的字幕，如果下载了字幕倒是没有刷新，则先刷新一次，便于后续的 Emby api 统计逻辑
	err := downloader.RefreshEmbySubList()
	if err != nil {
		log.Errorln("RefreshEmbySubList", err)
		return
	}
	err = downloader.GetUpdateVideoListFromEmby(config.MovieFolder, config.SeriesFolder)
	if err != nil {
		log.Errorln("GetUpdateVideoListFromEmby", err)
		return
	}
	// 开始下载，电影
	err = downloader.DownloadSub4Movie(config.MovieFolder)
	if err != nil {
		log.Errorln("DownloadSub4Movie", err)
		return
	}
	// 开始下载，连续剧
	err = downloader.DownloadSub4Series(config.SeriesFolder)
	if err != nil {
		log.Errorln("DownloadSub4Series", err)
		return
	}
	// 刷新 Emby 的字幕，下载完毕字幕了，就统一刷新一下
	err = downloader.RefreshEmbySubList()
	if err != nil {
		log.Errorln("RefreshEmbySubList", err)
		return
	}
}

var (
	log    *logrus.Logger
	config *types.Config
)
