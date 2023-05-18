.PHONY : build

default : build

build :
	go build -o bin/gittuf main.go

completion: build
	bin/gittuf completion bash > /usr/share/bash-completion/completions/gittuf

install : completion
	install -Dm 755 -t "/usr/local/bin" bin/gittuf

uninstall :
	rm /usr/local/bin/gittuf

clean :
	rm bin/gittuf
