package cookie

/**
* Session类，Session集合类
* 2016.09.30， 添加同一Session多站点应用的支持
 */

import (
	"container/list"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	// Sessions 全局单例SessionSet
	Sessions *SessionSet
	// Validity session有效时间
	Validity int
)

// SessionData 保存Seeeion数据
type SessionData map[string]interface{}

// Session 网页Session类
type Session struct {
	ProductInstance string                 //从当前URL获取product, instance
	ID              string                 //当前Session id
	Values          map[string]SessionData //Session 数据集
	Time            time.Time              //最新时间
	Element         *list.Element          // 在集合链的元素
	Sync            *sync.RWMutex          //多线程操作锁
}

// SessionSet 页面Session类集合
type SessionSet struct {
	Index  int64               // 当前Session建立数
	Values map[string]*Session // Session数据集合
	List   *list.List          // Session集合的有效顺序链，按有效时效最早到最晚
	Sync   *sync.RWMutex       //多线程操作锁
}

//启用一个全局变量Session集合
func init() {
	Sessions = &SessionSet{
		Index:  0,
		Values: make(map[string]*Session),
		List:   new(list.List),
		Sync:   new(sync.RWMutex),
	}
	Validity = 60 * 60
}

// Start 启用Session
func Start(w http.ResponseWriter, r *http.Request) *Session {
	name := "JCMS"
	key := ""
	//从请求获取cookie key
	cookie, err := r.Cookie(name)
	if err == nil {
		key = cookie.Value
	} else {
		//新生成cookie key
		key = Sessions.ID()
		c := &http.Cookie{
			Name:     name,
			Value:    key,
			Path:     "/",
			MaxAge:   Validity,
			HttpOnly: true,
		}
		http.SetCookie(w, c)
	}
	s := Sessions.Get(key)
	s.ProductInstance = ProductInstanceWithURL(r.URL.Path)
	return s
}

// ProductInstanceWithURL 从url地址获取产品与实例名称信息
func ProductInstanceWithURL(path string) string {
	if path == "" {
		return "JiangYouHua"
	}
	if path[0] == '/' && len(path) > 1 {
		path = path[1:]
	}
	a := strings.Split(path, "/")
	if len(a) == 1 {
		return a[0]
	}
	return fmt.Sprintf("%s-%s", a[0], a[1])
}

// Get 从SessionSet集合中获取session
func (ss *SessionSet) Get(key string) *Session {
	//已有
	if ss.Values == nil {
		return ss.Set(key)
	}
	ss.Sync.RLock()
	s, ok := ss.Values[key]
	ss.Sync.RUnlock()
	if ok {
		s.Time = time.Now()
		Sessions.List.MoveToBack(s.Element)
		return s
	}
	//没有全新设置
	return ss.Set(key)
}

// Set 向SessionSet集合添加新的Session
func (ss *SessionSet) Set(key string) *Session {
	if ss.Values == nil {
		ss.Values = make(map[string]*Session)
	}
	e := ss.List.PushBack(key)
	s := &Session{"", key, make(map[string]SessionData), time.Now(), e, new(sync.RWMutex)}
	if ss.Values == nil {
		ss.Values = make(map[string]*Session)
	}
	ss.Sync.Lock()
	ss.Values[key] = s
	ss.Sync.Unlock()
	ss.Index += int64(1)
	return s
}

// ID 生成SessionSet全局id
func (ss *SessionSet) ID() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		t := time.Now()
		nano := t.UnixNano()
		s := fmt.Sprintf("%d%v", nano, Sessions.Index)
		b = []byte(s)
	}
	ss.Index++
	return base64.URLEncoding.EncodeToString(b)
}

// Update 更新集合中的有效性，有效更新
func (ss *SessionSet) Update() {
	var n = new(list.Element)
	for e := ss.List.Front(); e != nil; e = n {
		n = e.Next()
		k, ok := e.Value.(string)
		if !ok || k == "" {
			ss.List.Remove(e)
		}
		val, ok := ss.Values[k]
		if !ok {
			ss.List.Remove(e)
			continue
		}
		if !val.Time.IsZero() && int(time.Now().Sub(val.Time).Seconds()) < Validity {
			return
		}
		ss.Sync.Lock()
		delete(ss.Values, k)
		ss.Sync.Unlock()
		ss.List.Remove(e)
	}
}

// UpdateAll 更新集合中的有效性，全部更新
func (ss *SessionSet) UpdateAll() {
	list := new(list.List)
	for key, val := range ss.Values {
		if !val.Time.IsZero() && int(time.Now().Sub(val.Time).Seconds()) < Validity {
			list.PushBack(val.Element)
			continue
		}
		ss.Sync.Lock()
		delete(ss.Values, key)
		ss.Sync.Unlock()
	}
	ss.List = list
}

// Get 获取保存在Session的内容
func (s *Session) Get(key string) interface{} {
	if s.Values == nil {
		return nil
	}
	// log.Println("0", s.Values)
	s.Sync.RLock()
	data, ok := s.Values[s.ProductInstance]
	s.Sync.RUnlock()
	if !ok {
		return nil
	}
	if v, ok := data[key]; ok {
		return v
	}
	return nil
}

// Set 保存内容在Session
func (s *Session) Set(key string, value interface{}) {
	if s.Values == nil {
		s.Values = make(map[string]SessionData)
	}
	data, ok := s.Values[s.ProductInstance]
	if !ok {
		data = make(SessionData)
		data[key] = value
		s.Values[s.ProductInstance] = data
		return
	}
	s.Sync.Lock()
	data[key] = value
	s.Sync.Unlock()
	s.Values[s.ProductInstance] = data
	// log.Println("1", s.Values)
}
