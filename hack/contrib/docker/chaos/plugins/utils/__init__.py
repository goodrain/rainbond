import ssl

try:
    _create_unverified_https_context = ssl._create_unverified_context
except AttributeError:
    pass
else:
    ssl._create_default_https_context = _create_unverified_https_context


def get_list_format(instance, attr):
    if hasattr(instance, attr):
        attribute = getattr(instance, attr)
        if isinstance(attribute, str):
            return [attribute]
        elif isinstance(attribute, (list, tuple)):
            return list(attribute)
        else:
            raise AttributeError(
                "expect <str, list, tuple> for '{0}', but got {1} of type {2}".format(attr, attribute, type(attribute)))
    else:
        return []
