import { describe, it, expect } from 'vitest';
import { AI_ERROR_RE, parseHostKeyMessage } from './detect';

describe('AI_ERROR_RE 报错检测', () => {
    it('行首 Error: 触发', () => {
        expect(AI_ERROR_RE.test('Error: something failed')).toBe(true);
    });

    it('多行输出中行首 Permission denied 触发', () => {
        expect(AI_ERROR_RE.test('line one\nPermission denied (publickey).')).toBe(true);
    });

    it('Traceback (Python) 触发', () => {
        expect(AI_ERROR_RE.test('Traceback (most recent call last):')).toBe(true);
    });

    it('中文认证失败触发', () => {
        expect(AI_ERROR_RE.test('认证失败: 密码错误')).toBe(true);
    });

    it('行中文档式 "error" 不触发 (避免误报)', () => {
        // "error" 出现在行中而非行首, 且非完整短语, 不应触发
        expect(AI_ERROR_RE.test('this readme mentions error handling')).toBe(false);
    });

    it('grep error 输出不触发 (error 不在行首)', () => {
        expect(AI_ERROR_RE.test('file.txt:1:some error here')).toBe(false);
    });

    it('正常输出不触发', () => {
        expect(AI_ERROR_RE.test('all systems operational')).toBe(false);
    });
});

describe('parseHostKeyMessage 主机密钥消息解析', () => {
    it('标准格式解析出 host/port/fingerprint', () => {
        const r = parseHostKeyMessage('HOST_KEY_UNVERIFIED|1.2.3.4|22|SHA256:abc');
        expect(r).not.toBeNull();
        expect(r!.host).toBe('1.2.3.4');
        expect(r!.port).toBe('22');
        expect(r!.fingerprint).toBe('SHA256:abc');
    });

    it('非主机密钥消息返回 null', () => {
        expect(parseHostKeyMessage('dial tcp: connection refused')).toBeNull();
    });

    it('字段不足返回 null', () => {
        expect(parseHostKeyMessage('HOST_KEY_UNVERIFIED|1.2.3.4')).toBeNull();
    });
});
