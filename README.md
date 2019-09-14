# qr-fns

Мета данные кассового чека по QR-коду их ФНС.

Данные из QR

```
t=20190418T211655&s=3943.26&fn=9282000100072197&i=64318&fp=2918241905&n=1
```

Проверка чека, в случае успеха возвращается 204 код
```
GET https://proverkacheka.nalog.ru:9999/v1/ofds/*/inns/*/fss/9282000100072197/operations/1/tickets/64318?fiscalSign=2918241905&date=2019-04-18T21:16:55&sum=394326
```

Получение данных

```
GET https://proverkacheka.nalog.ru:9999/v1/inns/*/kkts/*/fss/9282000100072197/tickets/64318?fiscalSign=2918241905&sendToEmail=no
```
