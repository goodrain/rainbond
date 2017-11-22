import re
import json
from addict import Dict


class JSON(object):

    @classmethod
    def dumps(cls, obj, indent=None):
        if isinstance(obj, str):
            return obj

        try:
            jstr = json.dumps(obj, separators=(',', ':'), indent=indent)
        except:
            jstr = None
        return jstr

    @classmethod
    def loads(cls, obj):
        try:
            obj = json.loads(obj)
        except:
            obj = None
        return obj


def to_dict(list_obj, dict_key):
    result = Dict({})
    for item in list_obj:
        result[item.pop(dict_key)] = item

    return result


def to_list(dict_obj, dict_key):
    result = []
    for k, v in dict_obj.items():
        item = Dict(v)
        item[dict_key] = k
        result.append(item)
    return result


class EncodeEscape(object):
    ESCAPE = re.compile(r'[\x00-\x1f\b\f\n\r\t]')
    ESCAPE_DCT = {
        '\b': '\\b',
        '\f': '\\f',
        '\n': '\\n',
        '\r': '\\r',
        '\t': '\\t',
    }

    for i in range(0x20):
        ESCAPE_DCT.setdefault(chr(i), '\\u{0:04x}'.format(i))

    @classmethod
    def encode(cls, s):
        def replace(match):
            return cls.ESCAPE_DCT[match.group(0)]
        return cls.ESCAPE.sub(replace, s)
