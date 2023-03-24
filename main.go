package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/wneessen/go-mail"
)

const (
	enviosParalelos = 10
	tamanhoBuffer   = 10
	duracaoTimeout  = 2 * time.Second
)

type remetente struct {
	nome, email, senha, host string
	porta                    int
}

type rabbit struct {
	user, senha, host, porta, vhost, fila string
}

type configuracoes struct {
	remetente
	rabbit
}

func pegarConfiguracoes() (*configuracoes, error) {
	err := godotenv.Load()
	if err != nil {
		return nil, err
	}

	porta, err := strconv.Atoi(os.Getenv("SMTP_PORT"))
	if err != nil {
		return nil, err
	}

	config := &configuracoes{
		remetente: remetente{
			nome:  os.Getenv("SMTP_USERNAME"),
			email: os.Getenv("SMTP_USER"),
			senha: os.Getenv("SMTP_PASSWORD"),
			host:  os.Getenv("SMTP_HOST"),
			porta: porta,
		},
		rabbit: rabbit{
			user:  os.Getenv("RABBIT_USER"),
			senha: os.Getenv("RABBIT_PASSWORD"),
			host:  os.Getenv("RABBIT_HOST"),
			porta: os.Getenv("RABBIT_PORT"),
			vhost: os.Getenv("RABBIT_VHOST"),
			fila:  os.Getenv("RABBIT_QUEUE"),
		},
	}

	return config, nil
}

func reenviarEmailsParaFila(descricao string, err error, emails []email) {
	log.Printf("[ERROR] - Erro ao processar um lote de emails, reenviando eles para a fila")
	log.Printf("[ERROR] - %s: %s", descricao, err)

	for _, email := range emails {
		err = email.mensagem.Nack(false, true)
		if err != nil {
			log.Printf("[ERROR] Erro ao reenviar mensagem para fila: %s", err)
		}
	}
}

func reenviarMensagemParaFila(descricao string, err error, mensagem amqp.Delivery) {
	log.Printf("[ERROR] - Erro ao processar a mensagem, reenviando ela para a fila")
	log.Printf("[ERROR] - %s: %s", descricao, err)

	err = mensagem.Nack(false, true)
	if err != nil {
		log.Printf("[ERROR] Erro ao reenviar mensagem para fila: %s", err)
	}
}

type destinatario struct {
	Nome, Email string
}

type email struct {
	Destinatario                destinatario
	Assunto, Mensagem, Template string
	Anexos                      []string
	mensagem                    amqp.Delivery
}

func enviarEmails(remetente remetente, fila []amqp.Delivery) {
	emails := []email{}

	for _, mensagem := range fila {
		email := email{}
		err := json.Unmarshal(mensagem.Body, &email)
		if err != nil {
			descricao := "Erro ao converter a mensagem para um email"
			reenviarMensagemParaFila(descricao, err, mensagem)
		} else {
			email.mensagem = mensagem
			emails = append(emails, email)
		}
	}

	opcoesCliente := []mail.Option{
		mail.WithPort(remetente.porta),
		mail.WithSMTPAuth(mail.SMTPAuthPlain),
		mail.WithUsername(remetente.email),
		mail.WithPassword(remetente.senha),
		mail.WithTLSPolicy(mail.TLSMandatory),
	}

	cliente, err := mail.NewClient(remetente.host, opcoesCliente...)
	if err != nil {
		descricao := "Erro ao criar um cliente de email"
		reenviarEmailsParaFila(descricao, err, emails)
		return
	}

	mensagens := []*mail.Msg{}
	emailsProcessados := []email{}
	for _, email := range emails {
		mensagem := mail.NewMsg()
		err = mensagem.EnvelopeFromFormat(remetente.nome, remetente.email)
		if err != nil {
			descricao := "Erro ao colocar remetente no email"
			reenviarMensagemParaFila(descricao, err, email.mensagem)
			continue
		}

		err = mensagem.AddToFormat(email.Destinatario.Nome, email.Destinatario.Email)
		if err != nil {
			descricao := "Erro ao colocar destinatario no email"
			reenviarMensagemParaFila(descricao, err, email.mensagem)
			continue
		}

		mensagem.Subject(email.Assunto)
		mensagem.SetBodyString(mail.TypeTextPlain, email.Mensagem)

		mensagens = append(mensagens, mensagem)
		emailsProcessados = append(emailsProcessados, email)
	}

	err = cliente.DialAndSend(mensagens...)
	if err != nil {
		descricao := "Erro ao enviar os emails"
		reenviarEmailsParaFila(descricao, err, emailsProcessados)
		return
	}

	quantidadeEnviados := 0
	for _, email := range emailsProcessados {
		err := email.mensagem.Ack(false)
		if err != nil {
			log.Printf("[ERRO] - Erro ao enviar mensagem de finalização para o rabbit: %s", err)
		} else {
			quantidadeEnviados += 1
		}
	}

	log.Printf("[INFO] - Foram enviado %d emails", quantidadeEnviados)
}

func main() {
	configuracoes, err := pegarConfiguracoes()
	if err != nil {
		log.Fatalf("[ERRO] - Erro ao ler as configurações: %v", err)
	}

	rabbitURL := fmt.Sprintf(
		"amqp://%s:%s@%s:%s/%s",
		configuracoes.rabbit.user,
		configuracoes.rabbit.senha,
		configuracoes.rabbit.host,
		configuracoes.rabbit.porta,
		configuracoes.rabbit.vhost,
	)

	rabbit, err := amqp.Dial(rabbitURL)
	if err != nil {
		log.Fatalf("[ERRO] - Erro ao conectar com o Rabbit: %s", err)
	}
	defer rabbit.Close()

	fila, err := rabbit.Channel()
	if err != nil {
		log.Fatalf("[ERRO] - Erro ao abrir o canal do Rabbit: %s", err)
	}
	defer fila.Close()

	err = fila.Qos(tamanhoBuffer*enviosParalelos, 0, false)
	if err != nil {
		log.Fatalf("[ERRO] - Erro ao configurar o tamanho da fila do consumidor: %s", err)
	}

	mensagens, err := fila.Consume(configuracoes.rabbit.fila, "", false, false, false, false, nil)
	if err != nil {
		log.Fatalf("[ERRO] - Erro ao registrar o consumidor: %s", err)
	}

	var esperar chan struct{}

	go func() {
		filaDeMensagens := []amqp.Delivery{}
		timeout := time.NewTicker(duracaoTimeout)

		for {
			select {
			case mensagen := <-mensagens:
				filaDeMensagens = append(filaDeMensagens, mensagen)
				timeout.Reset(duracaoTimeout)

				if len(filaDeMensagens) >= tamanhoBuffer {
					buffer := make([]amqp.Delivery, len(filaDeMensagens))
					copy(buffer, filaDeMensagens)
					log.Printf("[INFO] - Fazendo envio de %d emails", len(buffer))
					go enviarEmails(configuracoes.remetente, buffer)
					filaDeMensagens = filaDeMensagens[:0]
				}

			case <-timeout.C:
				if len(filaDeMensagens) > 0 {
					buffer := make([]amqp.Delivery, len(filaDeMensagens))
					copy(buffer, filaDeMensagens)
					log.Printf("[INFO] - Fazendo envio de %d emails", len(buffer))
					go enviarEmails(configuracoes.remetente, buffer)
					filaDeMensagens = filaDeMensagens[:0]
				}
			}
		}
	}()

	log.Printf("[INFO] - Servidor inicializado com sucesso")
	<-esperar
}
