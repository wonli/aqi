package utils

import (
	"encoding/json"
	"fmt"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	stream "github.com/alibabacloud-go/darabonba-stream/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
	"github.com/tidwall/gjson"
)

type AliOcr struct {
	uploadBody []byte
	apiParams  *openapi.Params

	client    *openapi.Client
	initError error

	response    any
	apiResponse any

	accessKey string
	secret    string
}

func NewAliOcr(accessKey, secret string) *AliOcr {
	instance := &AliOcr{
		accessKey: accessKey,
		secret:    secret,
	}

	//STS see:
	//https://help.aliyun.com/document_detail/378661.html
	client, err := instance.createClient(tea.String(accessKey), tea.String(secret))
	if err != nil {
		instance.initError = err
	}

	instance.client = client
	return instance
}

func (ali *AliOcr) Request() error {
	if ali.initError != nil {
		return ali.initError
	}

	if ali.uploadBody == nil {
		return fmt.Errorf("获取上传内容失败")
	}

	if ali.client == nil {
		return fmt.Errorf("初始化阿里客户端失败")
	}

	if ali.apiParams == nil {
		return fmt.Errorf("请求参数不能为空")
	}

	// runtime options
	runtime := &util.RuntimeOptions{}
	request := &openapi.OpenApiRequest{
		Stream: stream.ReadFromBytes(ali.uploadBody),
	}

	// 复制代码运行请自行打印 API 的返回值
	// 返回值为 Map 类型，可从 Map 中获得三类数据：响应体 body、响应头 headers、HTTP 返回的状态码 statusCode。
	res, err := ali.client.CallApi(ali.apiParams, request, runtime)
	if err != nil {
		return err
	}

	ali.apiResponse = res
	apiJson, err := json.Marshal(res)
	if err != nil {
		return err
	}

	statusCode := gjson.Get(string(apiJson), "statusCode").Int()
	if statusCode != 200 {
		return fmt.Errorf("返回状态码不正确")
	}

	if ali.response != nil {
		bodyData := gjson.Get(string(apiJson), "body.Data").String()
		err = json.Unmarshal([]byte(bodyData), ali.response)
		if err != nil {
			return err
		}
	}

	return nil
}

func (ali *AliOcr) WithResponse(s any) {
	ali.response = s
}

func (ali *AliOcr) WithApiName(apiName string) {
	//RecognizeDrivingLicense
	ali.apiParams = &openapi.Params{
		// 接口名称
		Action: tea.String(apiName),
		// 接口版本
		Version: tea.String("2021-07-07"),
		// 接口协议
		Protocol: tea.String("HTTPS"),
		// 接口 HTTP 方法
		Method:   tea.String("POST"),
		AuthType: tea.String("AK"),
		Style:    tea.String("V3"),
		// 接口 PATH
		Pathname: tea.String("/"),
		// 接口请求体内容格式
		ReqBodyType: tea.String("json"),
		// 接口响应体内容格式
		BodyType: tea.String("json"),
	}
}

func (ali *AliOcr) WithBody(body []byte) {
	ali.uploadBody = body
}

func (ali *AliOcr) createClient(accessKeyId *string, accessKeySecret *string) (res *openapi.Client, err error) {
	config := &openapi.Config{
		// 必填，您的 AccessKey ID
		AccessKeyId: accessKeyId,
		// 必填，您的 AccessKey Secret
		AccessKeySecret: accessKeySecret,
	}
	// 访问的域名
	config.Endpoint = tea.String("ocr-api.cn-hangzhou.aliyuncs.com")
	res = &openapi.Client{}
	res, err = openapi.NewClient(config)
	return res, err
}
