package ws

import (
	"fmt"
	"hash/fnv"
	"os"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/wonli/aqi/utils"

	"github.com/wonli/aqi/logger"
)

var langMap sync.Map

type langInfo struct {
	ctx *Context

	langData map[string]string
	filePath string
}

func languageInit(ctx *Context) *langInfo {
	lang := ctx.language
	value, ok := langMap.Load(lang)
	if ok {
		return value.(*langInfo)
	}

	languageFile := fmt.Sprintf("%s/i18n/%s.yaml", ctx.Server.dataPath, lang)
	err := utils.CreateFileIfNotExists(languageFile)
	if err != nil {
		return nil
	}

	file, err := os.ReadFile(languageFile)
	if err != nil {
		logger.SugarLog.Errorf("Failed to load language file:%s", err.Error())
		return nil
	}

	res := &langInfo{
		ctx:      ctx,
		langData: make(map[string]string),
		filePath: languageFile,
	}

	if len(file) == 0 {
		return res
	}

	err = yaml.Unmarshal(file, &res.langData)
	if err != nil {
		logger.SugarLog.Errorf("Failed to unmarshal language file:%s", err.Error())
		return nil
	}

	langMap.Store(lang, res)
	return res
}

func (info *langInfo) set(code int, msg string) {
	mHash := info.getMsgHashKey(msg)
	cacheKey := fmt.Sprintf("%s.%d.%s", info.ctx.Action, code, mHash)
	_, ok := info.langData[cacheKey]
	if !ok {
		info.langData[cacheKey] = msg

		// 将更新后的语言包数据写入文件
		if err := writeLangFile(info.filePath, info.langData); err != nil {
			logger.SugarLog.Errorf("Failed to update language file: %s", err.Error())
		}
	}
}

func (info *langInfo) load(code int, msg string) string {
	mHash := info.getMsgHashKey(msg)
	cacheKey := fmt.Sprintf("%s.%d.%s", info.ctx.Action, code, mHash)
	s, ok := info.langData[cacheKey]
	if ok {
		return s
	}

	return msg
}

func (info *langInfo) getMsgHashKey(msg string) string {
	h := fnv.New32a()
	_, err := h.Write([]byte(msg))
	if err != nil {
		logger.SugarLog.Errorf("Failed to retrieve hint information hash data:%s", err.Error())
		return "1"
	}

	return fmt.Sprintf("%04d", h.Sum32()%10000)
}

func writeLangFile(filePath string, data map[string]string) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := yaml.NewEncoder(file)
	return encoder.Encode(data)
}
