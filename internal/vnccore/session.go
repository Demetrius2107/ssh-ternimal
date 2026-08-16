// Package vnccore 内嵌 VNC 客户端: 自研精简 RFB 协议 (RFC 6143)
//
// 实现范围 (MVP): 版本协商 → 安全类型 (None / VNC Auth DES) → 初始化 →
// Raw 编码帧缓冲读取 → 键盘/鼠标输入转发。帧数据以 RGBA 输出供前端 canvas 渲染。
package vnccore

import (
	"bufio"
	"crypto/des"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"
)

// Session VNC 会话
type Session struct {
	conn   net.Conn
	r      *bufio.Reader
	w      sync.Mutex // 写锁 (输入事件)
	width  uint16
	height uint16
	pf     pixelFormat

	// frame 回调: 每次帧更新后携带完整 RGBA 缓冲 (w*h*4)
	frame func(width, height int, rgba []byte)
	stop  chan struct{}
	once  sync.Once
}

type pixelFormat struct {
	bpp     uint8
	depth   uint8
	bigEnd  bool
	trueCol bool
	rMax    uint16
	gMax    uint16
	bMax    uint16
	rShift  uint8
	gShift  uint8
	bShift  uint8
	pad     [3]byte
}

// Options 连接选项
type Options struct {
	Host     string
	Port     int
	Password string // 空 = 尝试无认证
}

// Connect 建立 VNC 连接并完成握手 (超时 10s)
func Connect(o Options) (*Session, error) {
	if o.Host == "" {
		return nil, errors.New("主机不能为空")
	}
	if o.Port <= 0 {
		o.Port = 5900
	}
	addr := fmt.Sprintf("%s:%d", o.Host, o.Port)
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("连接 %s 失败: %v", addr, err)
	}
	s := &Session{conn: conn, r: bufio.NewReader(conn), stop: make(chan struct{})}
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))

	if err := s.handshake(o.Password); err != nil {
		conn.Close()
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{}) // 握手完成清除 deadline
	return s, nil
}

// ---------- 握手 ----------

func (s *Session) handshake(password string) error {
	// 1. 版本协商: 发送 3.8, 接受服务器版本
	if _, err := s.conn.Write([]byte("RFB 003.008\n")); err != nil {
		return fmt.Errorf("发送版本失败: %v", err)
	}
	ver := make([]byte, 12)
	if _, err := io.ReadFull(s.r, ver); err != nil {
		return fmt.Errorf("读取版本失败: %v", err)
	}

	// 2. 安全类型: 服务器发 [count, types...]
	n, err := s.r.ReadByte()
	if err != nil {
		return fmt.Errorf("读取安全类型失败: %v", err)
	}
	types := make([]byte, int(n))
	if _, err := io.ReadFull(s.r, types); err != nil {
		return fmt.Errorf("读取安全类型列表失败: %v", err)
	}
	// 优先 VNC Auth (2), 其次 None (1)
	selected := byte(0)
	for _, t := range types {
		if t == 2 {
			selected = 2
			break
		}
		if t == 1 {
			selected = 1
		}
	}
	if selected == 0 {
		return errors.New("服务器不支持 None/VNC Auth 认证")
	}
	if _, err := s.conn.Write([]byte{selected}); err != nil {
		return fmt.Errorf("发送安全类型失败: %v", err)
	}

	if selected == 2 {
		// VNC Auth: 16 字节 challenge, DES 加密回传
		challenge := make([]byte, 16)
		if _, err := io.ReadFull(s.r, challenge); err != nil {
			return fmt.Errorf("读取 challenge 失败: %v", err)
		}
		if password == "" {
			return errors.New("服务器要求密码认证，但未提供密码")
		}
		enc, err := vncEncrypt(password, challenge)
		if err != nil {
			return err
		}
		if _, err := s.conn.Write(enc); err != nil {
			return fmt.Errorf("发送加密凭据失败: %v", err)
		}
		status := make([]byte, 4)
		if _, err := io.ReadFull(s.r, status); err != nil {
			return fmt.Errorf("读取认证结果失败: %v", err)
		}
		if binary.BigEndian.Uint32(status) != 0 {
			return errors.New("VNC 认证失败 (密码错误)")
		}
	} else {
		// None 认证: 3.8 需要读 4 字节状态
		status := make([]byte, 4)
		if _, err := io.ReadFull(s.r, status); err != nil {
			return fmt.Errorf("读取认证结果失败: %v", err)
		}
	}

	// 3. 客户端初始化: 共享标志
	if _, err := s.conn.Write([]byte{0}); err != nil {
		return fmt.Errorf("发送初始化失败: %v", err)
	}
	// 4. 服务器初始化: framebuffer 尺寸 + 像素格式 + 桌面名
	init := make([]byte, 24)
	if _, err := io.ReadFull(s.r, init); err != nil {
		return fmt.Errorf("读取服务器初始化失败: %v", err)
	}
	s.width = binary.BigEndian.Uint16(init[0:2])
	s.height = binary.BigEndian.Uint16(init[2:4])
	s.parsePixelFormat(init[4:20])
	nameLen := binary.BigEndian.Uint32(init[20:24])
	if nameLen > 0 && nameLen < 1024 {
		_, _ = io.CopyN(io.Discard, s.r, int64(nameLen))
	}
	return nil
}

// parsePixelFormat 解析 16 字节像素格式
func (s *Session) parsePixelFormat(b []byte) {
	s.pf = pixelFormat{
		bpp:     b[0],
		depth:   b[1],
		bigEnd:  b[2] != 0,
		trueCol: b[3] != 0,
		rMax:    binary.BigEndian.Uint16(b[4:6]),
		gMax:    binary.BigEndian.Uint16(b[6:8]),
		bMax:    binary.BigEndian.Uint16(b[8:10]),
		rShift:  b[10],
		gShift:  b[11],
		bShift:  b[12],
	}
}

// FrameHandler 设置帧回调 (Start 前调用)
func (s *Session) FrameHandler(fn func(width, height int, rgba []byte)) {
	s.frame = fn
}

// Start 启动帧读取循环 (阻塞, 通常 go 调用)
func (s *Session) Start() error {
	// 请求 Raw 编码
	if err := s.sendSetEncodings([]int32{0}); err != nil { // 0 = Raw
		return err
	}
	// 请求完整帧
	if err := s.sendFrameRequest(false, 0, 0, s.width, s.height); err != nil {
		return err
	}
	buf := make([]byte, int(s.width)*int(s.height)*4)
	for {
		select {
		case <-s.stop:
			return nil
		default:
		}
		updated, err := s.readFramebufferUpdate(buf)
		if err != nil {
			return err
		}
		if updated && s.frame != nil {
			s.frame(int(s.width), int(s.height), buf)
		}
	}
}

// readFramebufferUpdate 读取并应用一次帧更新, 返回是否有变更
func (s *Session) readFramebufferUpdate(buf []byte) (bool, error) {
	head := make([]byte, 4)
	if _, err := io.ReadFull(s.r, head); err != nil {
		return false, err
	}
	if head[0] != 0 { // FramebufferUpdate = 0
		return false, fmt.Errorf("意外消息类型 %d", head[0])
	}
	numRects := binary.BigEndian.Uint16(head[2:4])
	updated := false
	for i := 0; i < int(numRects); i++ {
		rh := make([]byte, 12)
		if _, err := io.ReadFull(s.r, rh); err != nil {
			return false, err
		}
		x := binary.BigEndian.Uint16(rh[0:2])
		y := binary.BigEndian.Uint16(rh[2:4])
		w := binary.BigEndian.Uint16(rh[4:6])
		h := binary.BigEndian.Uint16(rh[6:8])
		enc := int32(binary.BigEndian.Uint32(rh[8:12]))
		if enc == 0 { // Raw
			if err := s.readRawRect(buf, x, y, w, h); err != nil {
				return false, err
			}
			updated = true
		} else {
			// 未知编码: 跳过 (无法知道长度, 直接报错)
			return false, fmt.Errorf("不支持的编码 %d (仅支持 Raw)", enc)
		}
	}
	return updated, nil
}

// readRawRect 读取 Raw 编码矩形像素并写入缓冲
func (s *Session) readRawRect(buf []byte, x, y, w, h uint16) error {
	pixelBytes := int(s.pf.bpp) / 8
	n := int(w) * int(h) * pixelBytes
	px := make([]byte, n)
	if _, err := io.ReadFull(s.r, px); err != nil {
		return err
	}
	width := int(s.width)
	idx := 0
	for row := 0; row < int(h); row++ {
		dst := (int(y)+row)*width + int(x)
		for col := 0; col < int(w); col++ {
			raw := px[idx : idx+pixelBytes]
			idx += pixelBytes
			buf[dst*4] = s.red(raw)
			buf[dst*4+1] = s.green(raw)
			buf[dst*4+2] = s.blue(raw)
			buf[dst*4+3] = 255
			dst++
		}
	}
	return nil
}

// red/green/blue 按像素格式提取分量 (true color 走 mask/shift)
func (s *Session) red(raw []byte) byte {
	if !s.pf.trueCol {
		return raw[0]
	}
	v := s.pixelValue(raw)
	return byte(((v >> s.pf.rShift) & uint32(s.pf.rMax)) * 255 / uint32(s.pf.rMax))
}
func (s *Session) green(raw []byte) byte {
	if !s.pf.trueCol {
		return raw[1]
	}
	v := s.pixelValue(raw)
	return byte(((v >> s.pf.gShift) & uint32(s.pf.gMax)) * 255 / uint32(s.pf.gMax))
}
func (s *Session) blue(raw []byte) byte {
	if !s.pf.trueCol {
		return raw[2]
	}
	v := s.pixelValue(raw)
	return byte(((v >> s.pf.bShift) & uint32(s.pf.bMax)) * 255 / uint32(s.pf.bMax))
}

// pixelValue 像素原始值 (按大小端与位深)
func (s *Session) pixelValue(raw []byte) uint32 {
	var v uint32
	if s.pf.bigEnd {
		for _, b := range raw {
			v = v<<8 | uint32(b)
		}
	} else {
		for i := len(raw) - 1; i >= 0; i-- {
			v = v<<8 | uint32(raw[i])
		}
	}
	return v
}

// ---------- 输入转发 ----------

// KeyEvent 键盘事件: down=true 按下, keysym 为 X11 keysym (ASCII 字符直接用其码点)
func (s *Session) KeyEvent(keysym uint32, down bool) error {
	s.w.Lock()
	defer s.w.Unlock()
	msg := []byte{4, 0, 0, 0}
	if down {
		msg[1] = 1
	}
	msg = append(msg, byte(keysym>>24), byte(keysym>>16), byte(keysym>>8), byte(keysym))
	_, err := s.conn.Write(msg)
	return err
}

// PointerEvent 鼠标事件: buttonMask 1=左 2=中 4=右, 绝对坐标
func (s *Session) PointerEvent(buttonMask byte, x, y uint16) error {
	s.w.Lock()
	defer s.w.Unlock()
	msg := []byte{5, buttonMask, byte(x >> 8), byte(x), byte(y >> 8), byte(y)}
	_, err := s.conn.Write(msg)
	return err
}

// sendSetEncodings 设置编码列表
func (s *Session) sendSetEncodings(encs []int32) error {
	s.w.Lock()
	defer s.w.Unlock()
	msg := []byte{2, 0, byte(len(encs) >> 8), byte(len(encs))}
	for _, e := range encs {
		msg = append(msg, byte(e>>24), byte(e>>16), byte(e>>8), byte(e))
	}
	_, err := s.conn.Write(msg)
	return err
}

// sendFrameRequest 请求帧更新
func (s *Session) sendFrameRequest(inc bool, x, y, w, h uint16) error {
	s.w.Lock()
	defer s.w.Unlock()
	msg := []byte{3, 0, byte(x >> 8), byte(x), byte(y >> 8), byte(y), byte(w >> 8), byte(w), byte(h >> 8), byte(h)}
	if inc {
		msg[1] = 1
	}
	_, err := s.conn.Write(msg)
	return err
}

// Close 关闭连接
func (s *Session) Close() {
	s.once.Do(func() {
		close(s.stop)
		_ = s.conn.Close()
	})
}

// vncEncrypt VNC 认证: 密码 DES 加密 challenge (RFC 6143 §7.2.2)
// 密码截断/补齐到 8 字节, 每字节位反转作为密钥, 对 16 字节 challenge 分块加密
func vncEncrypt(password string, challenge []byte) ([]byte, error) {
	key := make([]byte, 8)
	copy(key, []byte(password))
	for i, b := range key {
		key[i] = reverseBits(b)
	}
	block, err := des.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("DES 初始化失败: %v", err)
	}
	out := make([]byte, 16)
	for i := 0; i < 2; i++ {
		block.Encrypt(out[i*8:(i+1)*8], challenge[i*8:(i+1)*8])
	}
	return out, nil
}

// reverseBits 位反转 (VNC 密码密钥需要)
func reverseBits(b byte) byte {
	var r byte
	for i := 0; i < 8; i++ {
		r = r<<1 | (b & 1)
		b >>= 1
	}
	return r
}
